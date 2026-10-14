// Package engine owns connections, poll definitions and the polling
// goroutines. It is UI-agnostic: state changes are pushed through an
// Emitter, which the Wails layer forwards to the frontend as events.
package engine

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"appmodbus/internal/datafmt"
	"appmodbus/internal/modbus"
	"appmodbus/internal/simulator"
)

// Event names emitted to the UI.
const (
	EventPollUpdate  = "poll:update"
	EventConnections = "connections"
	EventTraffic     = "traffic"
)

// Emitter delivers an event to the UI.
type Emitter func(event string, data any)

// ConnectionConfig is a user-defined connection.
type ConnectionConfig struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Mode       modbus.Mode `json:"mode"`
	SerialPort string      `json:"serialPort"`
	BaudRate   int         `json:"baudRate"`
	DataBits   int         `json:"dataBits"`
	Parity     string      `json:"parity"`
	StopBits   int         `json:"stopBits"`
	Host       string      `json:"host"`
	Port       int         `json:"port"`
	TimeoutMs  int         `json:"timeoutMs"` // response timeout
	DelayMs    int         `json:"delayMs"`   // minimum delay between requests
}

func (c ConnectionConfig) modbusConfig() modbus.Config {
	return modbus.Config{
		Mode: c.Mode, SerialPort: c.SerialPort, BaudRate: c.BaudRate, DataBits: c.DataBits,
		Parity: c.Parity, StopBits: c.StopBits, Host: c.Host, Port: c.Port,
	}
}

// ConnectionInfo is a connection plus its live state.
type ConnectionInfo struct {
	ConnectionConfig
	Summary   string `json:"summary"`
	Connected bool   `json:"connected"`
	Error     string `json:"error"`
}

// PollConfig defines one poll window (tab).
type PollConfig struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	ConnectionID  string                 `json:"connectionId"`
	SlaveID       int                    `json:"slaveId"`
	Function      int                    `json:"function"`
	Address       int                    `json:"address"` // protocol (0-based) address
	Quantity      int                    `json:"quantity"`
	ScanRateMs    int                    `json:"scanRateMs"`
	Format        datafmt.Format         `json:"format"`
	PLCAddressing bool                   `json:"plcAddressing"` // display 1-based
	CellFormats   map[int]datafmt.Format `json:"cellFormats"`
	Tags          map[int]Tag            `json:"tags"`
	Record        bool                   `json:"record"`    // store polled values on disk
	CommAlarm     bool                   `json:"commAlarm"` // alarm after repeated read failures
}

// Tag names a register and converts its raw value to engineering units:
// shown = raw × Gain + Offset.
type Tag struct {
	Name     string  `json:"name"`
	Gain     float64 `json:"gain"`
	Offset   float64 `json:"offset"`
	Unit     string  `json:"unit"`
	Decimals int     `json:"decimals"` // -1 = automatic
	Note     string  `json:"note,omitempty"`

	Alarm *AlarmLimits `json:"alarm,omitempty"`
}

// Scales reports whether the tag changes the value.
func (t Tag) Scales() bool { return t.Gain != 1 || t.Offset != 0 }

func normalizeTag(t *Tag) error {
	if t.Gain == 0 {
		t.Gain = 1
	}
	if t.Decimals < -1 || t.Decimals > 9 {
		return fmt.Errorf("decimals must be -1 (auto) or 0..9")
	}
	t.Name = strings.TrimSpace(t.Name)
	t.Unit = strings.TrimSpace(t.Unit)
	if t.Alarm != nil && !t.Alarm.Enabled && t.Alarm.HiHi == nil && t.Alarm.Hi == nil && t.Alarm.Lo == nil && t.Alarm.LoLo == nil && t.Alarm.OnState == nil {
		t.Alarm = nil
	}
	return normalizeAlarm(t.Alarm)
}

// decimalsFor picks display decimals: explicit, or derived from the gain
// (×0.1 → 1 decimal, ×0.01 → 2, …).
func (t Tag) decimalsFor(f datafmt.Format) int {
	if t.Decimals >= 0 {
		return t.Decimals
	}
	if !datafmt.IsInteger(f) {
		return -1
	}
	d := 0
	for g := math.Abs(t.Gain); g < 1 && g > 0 && d < 9; g *= 10 {
		d++
	}
	if t.Offset != math.Trunc(t.Offset) && d == 0 {
		d = 2
	}
	return d
}

// scaled formats raw × gain + offset.
func (t Tag) scaled(f datafmt.Format, regs []uint16) (string, bool) {
	v, ok := datafmt.Numeric(f, regs)
	if !ok {
		return "", false
	}
	v = v*t.Gain + t.Offset
	if d := t.decimalsFor(f); d >= 0 {
		return strconv.FormatFloat(v, 'f', d, 64), true
	}
	return strconv.FormatFloat(v, 'g', 7, 64), true
}

// Cell is one displayed value. Span is the number of registers the value
// covers; continuation cells of a multi-register value have Span 0.
type Cell struct {
	Address int    `json:"address"`
	Raw     int    `json:"raw"`
	Value   string `json:"value"`
	Span    int    `json:"span"`
	Format  string `json:"format"`
	Name    string `json:"name,omitempty"`
	Unit    string `json:"unit,omitempty"`
	RawText string `json:"rawText,omitempty"` // unscaled value when a tag scales it

	// Num is the value as a number in engineering units (after tag
	// scaling), for recording and charts. HasNum is false for cells
	// without data or with a non-finite value.
	Num    float64 `json:"num"`
	HasNum bool    `json:"hasNum"`

	Alarm string `json:"alarm,omitempty"` // alarm level, "" = normal
}

// PollState is the live state of a poll window.
type PollState struct {
	ID         string `json:"id"`
	Running    bool   `json:"running"`
	Tx         uint64 `json:"tx"`
	Errors     uint64 `json:"errors"`
	Valid      uint64 `json:"valid"`
	LastError  string `json:"lastError"`
	ResponseMs int64  `json:"responseMs"`
	UpdatedAt  int64  `json:"updatedAt"` // unix ms of last valid response
	HasData    bool   `json:"hasData"`
	Cells      []Cell `json:"cells"`
}

// Poll bundles a poll's configuration and state.
type Poll struct {
	Config PollConfig `json:"config"`
	State  PollState  `json:"state"`
}

type connection struct {
	cfg    ConnectionConfig
	client *modbus.Client
	err    string
	stats  *statsAcc
}

type poll struct {
	mu   sync.Mutex
	cfg  PollConfig
	st   PollState
	regs []uint16
	bits []bool
	stop chan struct{}
	kick chan struct{}
	done chan struct{}

	stats *statsAcc

	levels     map[int]string // alarm level per address (-1 = comm)
	failStreak int
}

// Engine is the application core.
type Engine struct {
	mu      sync.Mutex
	emit    Emitter
	conns   map[string]*connection
	polls   map[string]*poll
	order   []string
	nextID  int
	traffic *trafficLog
	scan    scanner
	alarms  alarmTable
	seq     sequencer

	// OnAlarm, when set, receives every alarm history event.
	OnAlarm func(AlarmEvent)

	Sim *simulator.Server

	// OnChange, if set, is called after any change to connections or poll
	// definitions (used to track unsaved workspace changes).
	OnChange func()

	// OnSample, if set, receives every successful read of a poll window
	// whose Record flag is on. cells must not be modified.
	OnSample func(cfg PollConfig, connection string, cells []Cell, t time.Time)
	loading  atomic.Bool
}

func (e *Engine) changed() {
	if e.OnChange != nil && !e.loading.Load() {
		e.OnChange()
	}
}

// New creates an engine; emit may be nil.
func New(emit Emitter) *Engine {
	if emit == nil {
		emit = func(string, any) {}
	}
	e := &Engine{
		emit:  emit,
		conns: map[string]*connection{},
		polls: map[string]*poll{},
		Sim:   simulator.NewServer(),
	}
	e.traffic = newTrafficLog(func(batch []TrafficEntry) { e.emit(EventTraffic, batch) })
	e.Sim.Traffic = e.traffic.observer("Simulator")
	return e
}

// Shutdown stops every poll, closes connections and the simulator.
func (e *Engine) Shutdown() {
	e.mu.Lock()
	polls := make([]*poll, 0, len(e.polls))
	for _, p := range e.polls {
		polls = append(polls, p)
	}
	e.mu.Unlock()
	for _, p := range polls {
		e.stopPoll(p)
	}
	e.mu.Lock()
	for _, c := range e.conns {
		_ = c.client.Close()
	}
	e.mu.Unlock()
	e.Sim.Stop()
	e.traffic.close()
}

func (e *Engine) newID(prefix string) string {
	e.nextID++
	return prefix + strconv.Itoa(e.nextID)
}

// ---- connections ----

func normalizeConn(c *ConnectionConfig) error {
	switch c.Mode {
	case modbus.ModeRTU, modbus.ModeASCII:
		if c.SerialPort == "" {
			return errors.New("select a serial port")
		}
		if c.BaudRate <= 0 {
			return errors.New("invalid baud rate")
		}
		if c.DataBits != 7 && c.DataBits != 8 {
			c.DataBits = 8
		}
		if c.Parity != "E" && c.Parity != "O" {
			c.Parity = "N"
		}
		if c.StopBits != 2 {
			c.StopBits = 1
		}
	case modbus.ModeTCP, modbus.ModeRTUOverTCP, modbus.ModeRTUOverUDP, modbus.ModeASCIITCP, modbus.ModeUDP:
		c.Host = strings.TrimSpace(c.Host)
		if c.Host == "" {
			return errors.New("enter an IP address or host name")
		}
		if c.Port < 1 || c.Port > 65535 {
			return errors.New("port must be 1..65535")
		}
	default:
		return fmt.Errorf("unknown connection type %q", c.Mode)
	}
	if c.TimeoutMs < 10 {
		c.TimeoutMs = 1000
	}
	if c.DelayMs < 0 {
		c.DelayMs = 0
	}
	if strings.TrimSpace(c.Name) == "" {
		c.Name = c.modbusConfig().Describe()
	}
	return nil
}

// SaveConnection creates (empty ID) or replaces a connection.
func (e *Engine) SaveConnection(cfg ConnectionConfig) (ConnectionInfo, error) {
	if err := normalizeConn(&cfg); err != nil {
		return ConnectionInfo{}, err
	}
	e.mu.Lock()
	if cfg.ID == "" {
		cfg.ID = e.newID("c")
	}
	old := e.conns[cfg.ID]
	e.mu.Unlock()

	tr, err := modbus.NewTransport(cfg.modbusConfig(), e.traffic.observer(cfg.Name))
	if err != nil {
		return ConnectionInfo{}, err
	}
	client := modbus.NewClient(tr)
	client.Gap = time.Duration(cfg.DelayMs) * time.Millisecond
	stats := newStatsAcc()
	if old != nil {
		_ = old.client.Close()
		stats = old.stats
	}
	client.Observe = func(_, _ byte, d time.Duration, err error) { stats.add(d, err) }
	e.mu.Lock()
	e.conns[cfg.ID] = &connection{cfg: cfg, client: client, stats: stats}
	e.mu.Unlock()
	e.emitConnections()
	e.changed()
	return e.connInfo(cfg.ID), nil
}

// DeleteConnection removes a connection; polls using it are stopped.
func (e *Engine) DeleteConnection(id string) {
	e.Disconnect(id)
	e.mu.Lock()
	delete(e.conns, id)
	e.mu.Unlock()
	e.emitConnections()
	e.changed()
}

// Connect opens a connection right away (polls also connect lazily).
func (e *Engine) Connect(id string) error {
	c := e.conn(id)
	if c == nil {
		return errors.New("unknown connection")
	}
	err := c.client.Connect()
	e.setConnErr(c, err)
	return err
}

// Disconnect stops the polls on a connection and closes it.
func (e *Engine) Disconnect(id string) {
	for _, p := range e.pollsOn(id) {
		e.stopPoll(p)
	}
	if c := e.conn(id); c != nil {
		_ = c.client.Close()
		e.setConnErr(c, nil)
	}
}

// Connections lists all connections with their state.
func (e *Engine) Connections() []ConnectionInfo {
	e.mu.Lock()
	ids := make([]string, 0, len(e.conns))
	for id := range e.conns {
		ids = append(ids, id)
	}
	e.mu.Unlock()
	sort.Slice(ids, func(i, j int) bool { return idLess(ids[i], ids[j]) })
	out := make([]ConnectionInfo, 0, len(ids))
	for _, id := range ids {
		out = append(out, e.connInfo(id))
	}
	return out
}

func idLess(a, b string) bool {
	na, _ := strconv.Atoi(a[1:])
	nb, _ := strconv.Atoi(b[1:])
	return na < nb
}

func (e *Engine) connInfo(id string) ConnectionInfo {
	e.mu.Lock()
	c := e.conns[id]
	e.mu.Unlock()
	if c == nil {
		return ConnectionInfo{}
	}
	return ConnectionInfo{
		ConnectionConfig: c.cfg,
		Summary:          c.cfg.modbusConfig().Describe(),
		Connected:        c.client.IsOpen(),
		Error:            e.connErr(c),
	}
}

func (e *Engine) conn(id string) *connection {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.conns[id]
}

func (e *Engine) connErr(c *connection) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return c.err
}

// setConnErr records the outcome of the latest request and emits a
// connection update when the visible state changed.
func (e *Engine) setConnErr(c *connection, err error) {
	msg := ""
	if err != nil && !isSlaveError(err) {
		msg = err.Error()
	}
	e.mu.Lock()
	changed := c.err != msg
	c.err = msg
	e.mu.Unlock()
	if changed || err == nil {
		e.emitConnections()
	}
}

// isSlaveError reports errors caused by the slave rather than the link.
func isSlaveError(err error) bool {
	var ex *modbus.ExceptionError
	return errors.As(err, &ex) || errors.Is(err, modbus.ErrTimeout) || errors.Is(err, modbus.ErrInvalidResponse)
}

func (e *Engine) emitConnections() { e.emit(EventConnections, e.Connections()) }

// ---- polls ----

func normalizePoll(p *PollConfig) error {
	switch byte(p.Function) {
	case modbus.FuncReadCoils, modbus.FuncReadDiscreteInputs:
		if p.Quantity < 1 || p.Quantity > modbus.MaxReadBits {
			return fmt.Errorf("quantity must be 1..%d", modbus.MaxReadBits)
		}
	case modbus.FuncReadHoldingRegisters, modbus.FuncReadInputRegisters:
		if p.Quantity < 1 || p.Quantity > modbus.MaxReadRegisters {
			return fmt.Errorf("quantity must be 1..%d", modbus.MaxReadRegisters)
		}
	default:
		return fmt.Errorf("function %d is not a read function", p.Function)
	}
	if p.SlaveID < 0 || p.SlaveID > 255 {
		return errors.New("slave ID must be 0..255")
	}
	if p.Address < 0 || p.Address+p.Quantity > 0x10000 {
		return errors.New("address range exceeds 0..65535")
	}
	if p.ScanRateMs < 10 {
		p.ScanRateMs = 10
	}
	if !datafmt.Valid(p.Format) {
		p.Format = datafmt.Signed
	}
	if p.CellFormats == nil {
		p.CellFormats = map[int]datafmt.Format{}
	}
	for a, f := range p.CellFormats {
		if !datafmt.Valid(f) {
			delete(p.CellFormats, a)
		}
	}
	if p.Tags == nil {
		p.Tags = map[int]Tag{}
	}
	for a, t := range p.Tags {
		if err := normalizeTag(&t); err != nil {
			return fmt.Errorf("tag at %d: %w", a, err)
		}
		p.Tags[a] = t
	}
	return nil
}

// CreatePoll adds a poll window. It is not started.
func (e *Engine) CreatePoll(cfg PollConfig) (Poll, error) {
	if err := normalizePoll(&cfg); err != nil {
		return Poll{}, err
	}
	e.mu.Lock()
	cfg.ID = e.newID("p")
	if strings.TrimSpace(cfg.Name) == "" {
		cfg.Name = "Mbpoll" + cfg.ID[1:]
	}
	p := &poll{cfg: cfg, st: PollState{ID: cfg.ID}, stats: newStatsAcc()}
	e.polls[cfg.ID] = p
	e.order = append(e.order, cfg.ID)
	e.mu.Unlock()
	p.mu.Lock()
	p.rebuildCells()
	snap := p.snapshot()
	p.mu.Unlock()
	e.changed()
	return snap, nil
}

// UpdatePoll replaces a poll's definition, restarting it if it was running.
func (e *Engine) UpdatePoll(cfg PollConfig) (Poll, error) {
	if err := normalizePoll(&cfg); err != nil {
		return Poll{}, err
	}
	p := e.poll(cfg.ID)
	if p == nil {
		return Poll{}, errors.New("unknown poll")
	}
	wasRunning := e.stopPoll(p)
	p.mu.Lock()
	reset := p.cfg.Function != cfg.Function || p.cfg.Address != cfg.Address ||
		p.cfg.Quantity != cfg.Quantity || p.cfg.SlaveID != cfg.SlaveID || p.cfg.ConnectionID != cfg.ConnectionID
	p.cfg = cfg
	if reset {
		p.regs, p.bits = nil, nil
		p.st = PollState{ID: cfg.ID}
	}
	p.rebuildCells()
	p.mu.Unlock()
	if wasRunning {
		if err := e.StartPoll(cfg.ID); err != nil {
			return e.pollSnapshot(p), err
		}
	}
	e.publish(p)
	e.changed()
	return e.pollSnapshot(p), nil
}

// DeletePoll stops and removes a poll window.
func (e *Engine) DeletePoll(id string) {
	p := e.poll(id)
	if p == nil {
		return
	}
	e.stopPoll(p)
	e.dropAlarms(id)
	e.mu.Lock()
	delete(e.polls, id)
	for i, o := range e.order {
		if o == id {
			e.order = append(e.order[:i], e.order[i+1:]...)
			break
		}
	}
	e.mu.Unlock()
	e.changed()
}

// Polls lists every poll in creation order.
func (e *Engine) Polls() []Poll {
	e.mu.Lock()
	ps := make([]*poll, 0, len(e.order))
	for _, id := range e.order {
		ps = append(ps, e.polls[id])
	}
	e.mu.Unlock()
	out := make([]Poll, len(ps))
	for i, p := range ps {
		out[i] = e.pollSnapshot(p)
	}
	return out
}

// SetCellFormat overrides the display format of the value at address.
// An empty format reverts the cell to the window's format.
func (e *Engine) SetCellFormat(id string, address int, f datafmt.Format) error {
	p := e.poll(id)
	if p == nil {
		return errors.New("unknown poll")
	}
	if f != "" && !datafmt.Valid(f) {
		return fmt.Errorf("unknown format %q", f)
	}
	p.mu.Lock()
	if f == "" || f == p.cfg.Format {
		delete(p.cfg.CellFormats, address)
	} else {
		p.cfg.CellFormats[address] = f
	}
	p.rebuildCells()
	p.mu.Unlock()
	e.publish(p)
	e.changed()
	return nil
}

// StartPoll starts the polling goroutine of a window.
func (e *Engine) StartPoll(id string) error {
	p := e.poll(id)
	if p == nil {
		return errors.New("unknown poll")
	}
	// Lock order is always e.mu before p.mu, so look the connection up first.
	p.mu.Lock()
	connID := p.cfg.ConnectionID
	p.mu.Unlock()
	if e.conn(connID) == nil {
		return errors.New("no connection selected; open Connection > Connect first")
	}
	p.mu.Lock()
	if p.stop != nil {
		p.mu.Unlock()
		return nil
	}
	p.stop = make(chan struct{})
	p.kick = make(chan struct{}, 1)
	p.done = make(chan struct{})
	p.st.Running = true
	stop, kick, done := p.stop, p.kick, p.done
	p.mu.Unlock()
	e.publish(p)
	go e.loop(p, stop, kick, done)
	return nil
}

// StopPoll stops a window's polling goroutine.
func (e *Engine) StopPoll(id string) {
	if p := e.poll(id); p != nil {
		e.stopPoll(p)
		e.publish(p)
	}
}

// StartAll and StopAll act on every window.
func (e *Engine) StartAll() error {
	var firstErr error
	for _, p := range e.Polls() {
		if err := e.StartPoll(p.Config.ID); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", p.Config.Name, err)
		}
	}
	return firstErr
}

func (e *Engine) StopAll() {
	for _, p := range e.Polls() {
		e.StopPoll(p.Config.ID)
	}
}

// ReadOnce performs a single read of a stopped window.
func (e *Engine) ReadOnce(id string) error {
	p := e.poll(id)
	if p == nil {
		return errors.New("unknown poll")
	}
	return e.pollOnce(p)
}

func (e *Engine) stopPoll(p *poll) (wasRunning bool) {
	p.mu.Lock()
	stop, done := p.stop, p.done
	p.stop, p.kick, p.done = nil, nil, nil
	p.st.Running = false
	p.mu.Unlock()
	if stop == nil {
		return false
	}
	close(stop)
	<-done
	return true
}

func (e *Engine) loop(p *poll, stop, kick <-chan struct{}, done chan struct{}) {
	defer close(done)
	for {
		start := time.Now()
		_ = e.pollOnce(p)
		p.mu.Lock()
		rate := time.Duration(p.cfg.ScanRateMs) * time.Millisecond
		p.mu.Unlock()
		wait := rate - time.Since(start)
		if wait < 0 {
			wait = 0
		}
		t := time.NewTimer(wait)
		select {
		case <-stop:
			t.Stop()
			return
		case <-kick:
			t.Stop()
		case <-t.C:
		}
	}
}

func (e *Engine) pollOnce(p *poll) error {
	p.mu.Lock()
	cfg := p.cfg
	p.mu.Unlock()
	c := e.conn(cfg.ConnectionID)
	if c == nil {
		err := errors.New("No connection")
		p.mu.Lock()
		p.st.LastError = err.Error()
		p.mu.Unlock()
		e.publish(p)
		return err
	}
	timeout := time.Duration(c.cfg.TimeoutMs) * time.Millisecond
	start := time.Now()
	bits, regs, elapsed, err := c.client.Read(byte(cfg.SlaveID), byte(cfg.Function), cfg.Address, cfg.Quantity, timeout)
	e.setConnErr(c, err)
	p.stats.add(elapsed, err)

	p.mu.Lock()
	p.st.Tx++
	if err != nil {
		p.st.Errors++
		p.st.LastError = describeErr(err)
	} else {
		p.st.Valid++
		p.st.LastError = ""
		p.st.ResponseMs = elapsed.Milliseconds()
		p.st.UpdatedAt = time.Now().UnixMilli()
		p.st.HasData = true
		p.regs, p.bits = regs, bits
		p.rebuildCells()
	}
	trans := p.evaluateAlarms(err, true)
	cells := p.st.Cells
	p.mu.Unlock()
	e.alarmsChanged(e.applyTransitions(trans))
	if err == nil && cfg.Record && e.OnSample != nil {
		e.OnSample(cfg, c.cfg.modbusConfig().Describe(), cells, start)
	}
	e.publish(p)
	return err
}

func describeErr(err error) string {
	switch {
	case errors.Is(err, modbus.ErrTimeout):
		return "Timeout Error"
	case strings.Contains(err.Error(), "CRC error"):
		return "CRC Error"
	}
	var ex *modbus.ExceptionError
	if errors.As(err, &ex) {
		return ex.Error()
	}
	if strings.Contains(err.Error(), "connection refused") {
		return "Connection refused"
	}
	return err.Error()
}

func (e *Engine) poll(id string) *poll {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.polls[id]
}

func (e *Engine) pollsOn(connID string) []*poll {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []*poll
	for _, p := range e.polls {
		p.mu.Lock()
		if p.cfg.ConnectionID == connID {
			out = append(out, p)
		}
		p.mu.Unlock()
	}
	return out
}

func (e *Engine) pollSnapshot(p *poll) Poll {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.snapshot()
}

func (e *Engine) publish(p *poll) { e.emit(EventPollUpdate, e.pollSnapshot(p)) }

// snapshot copies the poll; caller holds p.mu.
func (p *poll) snapshot() Poll {
	cfg := p.cfg
	cfg.CellFormats = make(map[int]datafmt.Format, len(p.cfg.CellFormats))
	for k, v := range p.cfg.CellFormats {
		cfg.CellFormats[k] = v
	}
	cfg.Tags = make(map[int]Tag, len(p.cfg.Tags))
	for k, v := range p.cfg.Tags {
		cfg.Tags[k] = v
	}
	st := p.st
	st.Cells = append([]Cell(nil), p.st.Cells...)
	return Poll{Config: cfg, State: st}
}

// rebuildCells lays out the display cells from the last data; caller holds p.mu.
func (p *poll) rebuildCells() {
	cfg := p.cfg
	cells := make([]Cell, cfg.Quantity)
	if modbus.IsBitFunction(byte(cfg.Function)) {
		for i := range cells {
			c := Cell{Address: cfg.Address + i, Span: 1, Format: "bit", Value: "-"}
			if t, ok := cfg.Tags[c.Address]; ok {
				c.Name, c.Unit = t.Name, t.Unit
			}
			if i < len(p.bits) {
				c.Raw = b2i(p.bits[i])
				c.Value = strconv.Itoa(c.Raw)
				c.Num, c.HasNum = float64(c.Raw), true
			}
			cells[i] = c
		}
		p.st.Cells = cells
		p.applyLevels()
		return
	}
	for i := 0; i < cfg.Quantity; {
		addr := cfg.Address + i
		f := cfg.Format
		if cf, ok := cfg.CellFormats[addr]; ok {
			f = cf
		}
		span := datafmt.Registers(f)
		if i+span > cfg.Quantity {
			span = cfg.Quantity - i // not enough registers left: show what fits
		}
		c := Cell{Address: addr, Span: span, Format: string(f), Value: "-"}
		tag, tagged := cfg.Tags[addr]
		if tagged {
			c.Name, c.Unit = tag.Name, tag.Unit
		}
		if i < len(p.regs) {
			c.Raw = int(p.regs[i])
			if span == datafmt.Registers(f) && i+span <= len(p.regs) {
				regs := p.regs[i : i+span]
				c.Value = datafmt.FormatValue(f, regs)
				if n, ok := datafmt.Numeric(f, regs); ok {
					c.Num, c.HasNum = n, true
				}
				if tagged && tag.Scales() {
					if v, ok := tag.scaled(f, regs); ok {
						c.RawText, c.Value = c.Value, v
						c.Num = c.Num*tag.Gain + tag.Offset
					}
				}
			} else {
				c.Value = "—"
			}
		}
		cells[i] = c
		for k := 1; k < span; k++ {
			cc := Cell{Address: addr + k, Span: 0, Format: string(f)}
			if i+k < len(p.regs) {
				cc.Raw = int(p.regs[i+k])
			}
			cells[i+k] = cc
		}
		i += span
	}
	p.st.Cells = cells
	p.applyLevels()
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// SetRecord turns recording of a poll window on or off.
func (e *Engine) SetRecord(id string, on bool) error {
	p := e.poll(id)
	if p == nil {
		return errors.New("unknown poll")
	}
	p.mu.Lock()
	changed := p.cfg.Record != on
	p.cfg.Record = on
	p.mu.Unlock()
	if changed {
		e.publish(p)
		e.changed()
	}
	return nil
}
