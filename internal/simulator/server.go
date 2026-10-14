package simulator

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.bug.st/serial"

	"appmodbus/internal/datafmt"
	"appmodbus/internal/modbus"
)

// Config selects how the simulated slave listens.
type Config struct {
	Mode       modbus.Mode `json:"mode"`
	SerialPort string      `json:"serialPort"`
	BaudRate   int         `json:"baudRate"`
	DataBits   int         `json:"dataBits"`
	Parity     string      `json:"parity"`
	StopBits   int         `json:"stopBits"`
	Host       string      `json:"host"` // listen address, "" = all interfaces
	Port       int         `json:"port"`
	SlaveID    int         `json:"slaveId"`
	// Animate makes input registers 0..9 and discrete inputs 0..7 change
	// continuously so pollers have live data to show.
	Animate bool `json:"animate"`
	// Limits restricts each table to address ranges such as "0-99,200-219"
	// (missing or blank = all 65536 addresses).
	Limits map[Table]string `json:"limits,omitempty"`
}

// Status is reported to the UI.
type Status struct {
	Running  bool   `json:"running"`
	Config   Config `json:"config"`
	Requests uint64 `json:"requests"`
	Clients  int    `json:"clients"`
	Error    string `json:"error"`
}

// Server runs one simulated slave on top of a Store.
type Server struct {
	Store   *Store
	Traffic modbus.TrafficFunc

	mu       sync.Mutex
	cfg      Config
	running  bool
	stop     chan struct{}
	closers  []io.Closer
	conns    map[net.Conn]struct{}
	wg       sync.WaitGroup
	requests atomic.Uint64
	clients  atomic.Int32
	lastErr  atomic.Value
	addr     string

	// Serial-line diagnostic counters (FC08).
	counters [8]atomic.Uint32
}

// Indexes into Server.counters, in FC08 sub-function order (0x0B..0x12).
const (
	cntBusMsg = iota
	cntCommErr
	cntException
	cntSlaveMsg
	cntNoResponse
	cntNAK
	cntBusy
	cntOverrun
)

// NewServer creates a stopped server with an empty store.
func NewServer() *Server { return &Server{Store: NewStore()} }

func (s *Server) traffic(d modbus.Direction, b []byte) {
	if s.Traffic != nil {
		s.Traffic(d, b)
	}
}

// Start begins serving. It fails if the listener cannot be opened.
func (s *Server) Start(cfg Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return errors.New("simulator already running")
	}
	if cfg.SlaveID < 1 || cfg.SlaveID > 247 {
		return fmt.Errorf("slave ID %d out of range 1..247", cfg.SlaveID)
	}
	if err := s.Store.SetLimits(cfg.Limits); err != nil {
		return err
	}
	s.clearCounters()
	s.stop = make(chan struct{})
	s.closers = nil
	s.conns = map[net.Conn]struct{}{}
	s.lastErr.Store("")
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	switch cfg.Mode {
	case modbus.ModeTCP, modbus.ModeRTUOverTCP, modbus.ModeASCIITCP:
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		s.closers = append(s.closers, ln)
		s.addr = ln.Addr().String()
		s.wg.Add(1)
		go s.acceptLoop(ln, cfg)
	case modbus.ModeRTUOverUDP, modbus.ModeUDP:
		pc, err := net.ListenPacket("udp", addr)
		if err != nil {
			return err
		}
		s.closers = append(s.closers, pc)
		s.addr = pc.LocalAddr().String()
		s.wg.Add(1)
		go s.serveUDP(pc, cfg)
	case modbus.ModeRTU, modbus.ModeASCII:
		mode := &serial.Mode{BaudRate: cfg.BaudRate, DataBits: cfg.DataBits, Parity: serial.NoParity, StopBits: serial.OneStopBit}
		switch cfg.Parity {
		case "E":
			mode.Parity = serial.EvenParity
		case "O":
			mode.Parity = serial.OddParity
		}
		if cfg.StopBits == 2 {
			mode.StopBits = serial.TwoStopBits
		}
		if mode.DataBits == 0 {
			mode.DataBits = 8
		}
		p, err := serial.Open(cfg.SerialPort, mode)
		if err != nil {
			return fmt.Errorf("open %s: %w%s", cfg.SerialPort, err, modbus.SerialPermissionHint(err))
		}
		s.closers = append(s.closers, p)
		s.addr = cfg.SerialPort
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			read := func(b []byte, t time.Duration) (int, error) {
				if err := p.SetReadTimeout(t); err != nil {
					return 0, err
				}
				return p.Read(b)
			}
			if cfg.Mode == modbus.ModeASCII {
				s.serveASCIIStream(read, p, cfg)
				return
			}
			s.serveRTUStream(read, p, cfg)
		}()
	default:
		return fmt.Errorf("unknown mode %q", cfg.Mode)
	}
	if cfg.Animate {
		s.wg.Add(1)
		go s.animate()
	}
	s.cfg = cfg
	s.running = true
	return nil
}

// Stop shuts the server down and waits for its goroutines.
func (s *Server) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	close(s.stop)
	for _, c := range s.closers {
		_ = c.Close()
	}
	for c := range s.conns {
		_ = c.Close()
	}
	s.running = false
	s.mu.Unlock()
	s.wg.Wait()
}

// Addr returns the address actually listened on (useful with port 0).
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// Status returns the current state.
func (s *Server) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, _ := s.lastErr.Load().(string)
	return Status{Running: s.running, Config: s.cfg, Requests: s.requests.Load(), Clients: int(s.clients.Load()), Error: e}
}

func (s *Server) stopped() bool {
	select {
	case <-s.stop:
		return true
	default:
		return false
	}
}

// handle runs a request addressed to unit. ok is false when no reply
// must be sent (other slave or broadcast).
func (s *Server) handle(cfg Config, unit byte, req modbus.PDU, tcp bool) (modbus.PDU, bool) {
	s.counters[cntBusMsg].Add(1)
	if unit != 0 && int(unit) != cfg.SlaveID {
		return modbus.PDU{}, false
	}
	s.requests.Add(1)
	s.counters[cntSlaveMsg].Add(1)
	resp := s.respond(req)
	if resp.Function&0x80 != 0 {
		s.counters[cntException].Add(1)
	}
	if unit == 0 && !tcp {
		s.counters[cntNoResponse].Add(1)
		return modbus.PDU{}, false // serial broadcast: no reply
	}
	return resp, true // over TCP unit 0 is a normal request
}

func (s *Server) respond(req modbus.PDU) modbus.PDU {
	switch req.Function {
	case modbus.FuncDiagnostics:
		return s.diagnostics(req)
	case modbus.FuncEncapsulated:
		return deviceIdentification(req)
	}
	return s.Store.Handle(req)
}

func (s *Server) clearCounters() {
	for i := range s.counters {
		s.counters[i].Store(0)
	}
}

// diagnostics answers FC08: echo, restart, counters.
func (s *Server) diagnostics(req modbus.PDU) modbus.PDU {
	fc := req.Function
	if len(req.Data) != 4 {
		return exception(fc, modbus.ExIllegalDataValue)
	}
	sub := uint16(req.Data[0])<<8 | uint16(req.Data[1])
	switch sub {
	case modbus.DiagReturnQueryData:
		return modbus.PDU{Function: fc, Data: append([]byte(nil), req.Data...)}
	case modbus.DiagRestartComm, modbus.DiagClearCounters:
		s.clearCounters()
		return modbus.PDU{Function: fc, Data: append([]byte(nil), req.Data...)}
	}
	if sub >= modbus.DiagBusMessageCount && sub <= modbus.DiagBusCharOverrunCount {
		v := s.counters[int(sub-modbus.DiagBusMessageCount)].Load()
		return modbus.PDU{Function: fc, Data: []byte{req.Data[0], req.Data[1], byte(v >> 8), byte(v)}}
	}
	return exception(fc, modbus.ExIllegalFunction)
}

// simIdentity is what the simulator reports for FC43/14.
var simIdentity = []modbus.DeviceObject{
	{ID: 0x00, Value: "AppMODBUS"},
	{ID: 0x01, Value: "SIM-1"},
	{ID: 0x02, Value: "1.0"},
	{ID: 0x03, Value: "https://github.com/elmnaveen/AppMODBUS"},
	{ID: 0x04, Value: "AppMODBUS slave simulator"},
	{ID: 0x05, Value: "Simulator"},
	{ID: 0x06, Value: "Demo meter"},
}

func deviceIdentification(req modbus.PDU) modbus.PDU {
	fc := req.Function
	d := req.Data
	if len(d) != 3 || d[0] != modbus.MEIReadDeviceID {
		return exception(fc, modbus.ExIllegalFunction)
	}
	code, obj := d[1], d[2]
	last := byte(0x02) // basic
	switch code {
	case modbus.DeviceIDBasic:
	case modbus.DeviceIDRegular, modbus.DeviceIDExtended:
		last = 0x06
	case modbus.DeviceIDSpecific:
		for _, o := range simIdentity {
			if o.ID == obj {
				return modbus.EncodeDeviceID(code, 0x82, []modbus.DeviceObject{o})
			}
		}
		return exception(fc, modbus.ExIllegalDataAddress)
	default:
		return exception(fc, modbus.ExIllegalDataValue)
	}
	if obj > last {
		obj = 0 // the spec restarts from 0 for an unknown object
	}
	var objs []modbus.DeviceObject
	for _, o := range simIdentity {
		if o.ID >= obj && o.ID <= last {
			objs = append(objs, o)
		}
	}
	return modbus.EncodeDeviceID(code, 0x82, objs)
}

func (s *Server) acceptLoop(ln net.Listener, cfg Config) {
	defer s.wg.Done()
	var conns sync.WaitGroup
	defer conns.Wait()
	for {
		c, err := ln.Accept()
		if err != nil {
			if !s.stopped() {
				s.lastErr.Store(err.Error())
			}
			return
		}
		s.mu.Lock()
		s.conns[c] = struct{}{}
		s.mu.Unlock()
		conns.Add(1)
		s.clients.Add(1)
		go func() {
			defer conns.Done()
			defer s.clients.Add(-1)
			defer func() {
				s.mu.Lock()
				delete(s.conns, c)
				s.mu.Unlock()
				c.Close()
			}()
			if cfg.Mode == modbus.ModeTCP {
				s.serveMBAP(c, cfg)
				return
			}
			read := func(b []byte, t time.Duration) (int, error) {
				_ = c.SetReadDeadline(time.Now().Add(t))
				n, err := c.Read(b)
				var ne net.Error
				if errors.As(err, &ne) && ne.Timeout() {
					return n, nil
				}
				return n, err
			}
			if cfg.Mode == modbus.ModeASCIITCP {
				s.serveASCIIStream(read, c, cfg)
				return
			}
			s.serveRTUStream(read, c, cfg)
		}()
	}
}

func (s *Server) serveMBAP(c net.Conn, cfg Config) {
	buf := make([]byte, 260)
	for !s.stopped() {
		_ = c.SetReadDeadline(time.Time{})
		if _, err := io.ReadFull(c, buf[:modbus.MBAPHeaderLen]); err != nil {
			return
		}
		h, err := modbus.DecodeMBAPHeader(buf[:modbus.MBAPHeaderLen])
		if err != nil {
			s.traffic(modbus.Rx, buf[:modbus.MBAPHeaderLen])
			return
		}
		end := modbus.MBAPHeaderLen + int(h.Length) - 1
		if _, err := io.ReadFull(c, buf[modbus.MBAPHeaderLen:end]); err != nil {
			return
		}
		s.traffic(modbus.Rx, buf[:end])
		req := modbus.PDU{Function: buf[modbus.MBAPHeaderLen], Data: append([]byte(nil), buf[modbus.MBAPHeaderLen+1:end]...)}
		resp, ok := s.handle(cfg, h.Unit, req, true)
		if !ok {
			// A real gateway would answer exception 0B; a lone slave stays silent.
			s.reply(c, modbus.EncodeMBAP(h.TxID, h.Unit, exception(req.Function, modbus.ExGatewayNoResponse)))
			continue
		}
		s.reply(c, modbus.EncodeMBAP(h.TxID, h.Unit, resp))
	}
}

func (s *Server) reply(w io.Writer, adu []byte) {
	s.traffic(modbus.Tx, adu)
	_, _ = w.Write(adu)
}

// serveRTUStream reads RTU frames from a byte stream. read must return
// (0, nil) when nothing arrives within the timeout.
func (s *Server) serveRTUStream(read func([]byte, time.Duration) (int, error), w io.Writer, cfg Config) {
	const interByte = 50 * time.Millisecond
	buf := make([]byte, 300)
	for !s.stopped() {
		n, err := read(buf[:1], 200*time.Millisecond)
		if err != nil {
			return
		}
		if n == 0 {
			continue
		}
		for {
			total, need := modbus.RTURequestLength(buf[:n])
			target := total
			if target == 0 {
				if n >= need {
					break // unknown function code
				}
				target = need
			}
			if n >= target || target > len(buf) {
				break
			}
			m, err := read(buf[n:target], interByte)
			if err != nil {
				return
			}
			if m == 0 {
				break // incomplete frame; CRC check below rejects it
			}
			n += m
		}
		if total, _ := modbus.RTURequestLength(buf[:n]); total == 0 {
			// Unknown function code: gather the rest until the line goes quiet.
			for n < len(buf) {
				m, err := read(buf[n:], interByte)
				if err != nil {
					return
				}
				if m == 0 {
					break
				}
				n += m
			}
		}
		frame := buf[:n]
		s.traffic(modbus.Rx, frame)
		unit, req, err := modbus.DecodeRTU(frame)
		if err != nil {
			s.counters[cntCommErr].Add(1)
			continue // corrupt frame: a slave stays silent
		}
		if resp, ok := s.handle(cfg, unit, req, false); ok {
			s.reply(w, modbus.EncodeRTU(unit, resp))
		}
	}
}

// serveASCIIStream reads ':'-delimited ASCII frames from a byte stream.
// read must return (0, nil) when nothing arrives within the timeout.
func (s *Server) serveASCIIStream(read func([]byte, time.Duration) (int, error), w io.Writer, cfg Config) {
	frame := make([]byte, 0, 600)
	buf := make([]byte, 256)
	for !s.stopped() {
		n, err := read(buf, 200*time.Millisecond)
		if err != nil {
			return
		}
		for _, c := range buf[:n] {
			switch {
			case c == ':':
				frame = append(frame[:0], c)
			case len(frame) == 0:
			default:
				frame = append(frame, c)
				if len(frame) > 600 {
					frame = frame[:0]
					s.counters[cntCommErr].Add(1)
					continue
				}
				if c != '\n' {
					continue
				}
				s.traffic(modbus.Rx, frame)
				unit, req, err := modbus.DecodeASCII(frame)
				frame = frame[:0]
				if err != nil {
					s.counters[cntCommErr].Add(1)
					continue
				}
				if resp, ok := s.handle(cfg, unit, req, false); ok {
					s.reply(w, modbus.EncodeASCII(unit, resp))
				}
			}
		}
	}
}

func (s *Server) serveUDP(pc net.PacketConn, cfg Config) {
	defer s.wg.Done()
	buf := make([]byte, 512)
	for {
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			if !s.stopped() {
				s.lastErr.Store(err.Error())
			}
			return
		}
		s.traffic(modbus.Rx, buf[:n])
		if cfg.Mode == modbus.ModeUDP {
			s.serveMBAPDatagram(pc, from, buf[:n], cfg)
			continue
		}
		unit, req, err := modbus.DecodeRTU(buf[:n])
		if err != nil {
			s.counters[cntCommErr].Add(1)
			continue
		}
		if resp, ok := s.handle(cfg, unit, req, false); ok {
			adu := modbus.EncodeRTU(unit, resp)
			s.traffic(modbus.Tx, adu)
			_, _ = pc.WriteTo(adu, from)
		}
	}
}

func (s *Server) animate() {
	defer s.wg.Done()
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	start := time.Now()
	walk := []int{2300, 2310, 2295, 500, 498, 503, 4999}
	for {
		select {
		case <-s.stop:
			return
		case now := <-t.C:
			sec := now.Sub(start).Seconds()
			// IR 0-1: float (ABCD) sine wave around 230 V.
			regs, _ := datafmt.ParseValue("float_abcd", strconv.FormatFloat(230+5*math.Sin(sec/3), 'f', 3, 32))
			vals := []int{int(regs[0]), int(regs[1]), int(uint16(sec * 2))}
			for i := range walk {
				walk[i] += rand.Intn(7) - 3
				vals = append(vals, walk[i])
			}
			s.Store.Write(InputRegisters, 0, vals)
			bits := make([]int, 8)
			step := int(sec*2) % 8
			for i := range bits {
				bits[i] = b2i(i == step)
			}
			s.Store.Write(DiscreteInputs, 0, bits)
		}
	}
}

// serveMBAPDatagram answers one Modbus UDP request.
func (s *Server) serveMBAPDatagram(pc net.PacketConn, from net.Addr, b []byte, cfg Config) {
	if len(b) < modbus.MBAPHeaderLen+1 {
		return
	}
	h, err := modbus.DecodeMBAPHeader(b[:modbus.MBAPHeaderLen])
	if err != nil || modbus.MBAPHeaderLen+int(h.Length)-1 != len(b) {
		return
	}
	req := modbus.PDU{Function: b[modbus.MBAPHeaderLen], Data: append([]byte(nil), b[modbus.MBAPHeaderLen+1:]...)}
	resp, ok := s.handle(cfg, h.Unit, req, true)
	if !ok {
		resp = exception(req.Function, modbus.ExGatewayNoResponse)
	}
	adu := modbus.EncodeMBAP(h.TxID, h.Unit, resp)
	s.traffic(modbus.Tx, adu)
	_, _ = pc.WriteTo(adu, from)
}
