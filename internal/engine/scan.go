package engine

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"modbusstudio/internal/modbus"
)

// EventScan carries ScanState updates.
const EventScan = "scan"

// Scan kinds.
const (
	ScanIDs     = "ids"
	ScanMap     = "map"
	ScanNetwork = "network"
)

// IDResult is a slave that answered an ID scan.
type IDResult struct {
	SlaveID   int     `json:"slaveId"`
	Status    string  `json:"status"` // ok, exception, garbled
	Exception string  `json:"exception"`
	Ms        float64 `json:"ms"`
}

// MapRange is a run of readable addresses found by a map scan (inclusive).
type MapRange struct {
	Function int `json:"function"`
	From     int `json:"from"`
	To       int `json:"to"`
}

// NetResult is a host found by a network scan.
type NetResult struct {
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	Units     []int    `json:"units"`     // unit IDs that answered Modbus requests
	Status    string   `json:"status"`    // modbus, open (port open, no Modbus answer)
	Exception string   `json:"exception"` // last exception seen, if any
	Ms        float64  `json:"ms"`        // connect time
	Identity  []string `json:"identity"`  // vendor, product code, revision (FC43)
}

// ScanState is the progress and results of the current or last scan.
type ScanState struct {
	Kind        string      `json:"kind"`
	Running     bool        `json:"running"`
	Done        int         `json:"done"`
	Total       int         `json:"total"`
	Phase       string      `json:"phase"`
	Started     int64       `json:"started"`
	Finished    int64       `json:"finished"`
	Error       string      `json:"error"`
	IDs         []IDResult  `json:"ids"`
	Ranges      []MapRange  `json:"ranges"`
	Unsupported []int       `json:"unsupported"` // functions answered with "illegal function"
	Hosts       []NetResult `json:"hosts"`
	SlaveID     int         `json:"slaveId"`
	Connection  string      `json:"connectionId"`
	Seq         uint64      `json:"seq"` // increases with every update
}

type scanner struct {
	emitMu   sync.Mutex // keeps updates in order
	mu       sync.Mutex
	st       ScanState
	cancel   context.CancelFunc
	lastEmit time.Time
}

// IDScanRequest scans slave IDs on one connection.
type IDScanRequest struct {
	ConnectionID string `json:"connectionId"`
	From         int    `json:"from"`
	To           int    `json:"to"`
	Function     int    `json:"function"` // read function used as probe (1-4)
	Address      int    `json:"address"`
	TimeoutMs    int    `json:"timeoutMs"`
}

// MapScanRequest probes which addresses of one slave can be read.
type MapScanRequest struct {
	ConnectionID string `json:"connectionId"`
	SlaveID      int    `json:"slaveId"`
	Functions    []int  `json:"functions"`
	From         int    `json:"from"`
	To           int    `json:"to"`
	TimeoutMs    int    `json:"timeoutMs"`
}

// NetworkScanRequest looks for Modbus TCP devices in an IPv4 range.
type NetworkScanRequest struct {
	Range     string `json:"range"` // 192.168.1.1-254, 10.0.0.0/24, a-b, or one address
	Port      int    `json:"port"`
	Units     string `json:"units"` // unit IDs to try, e.g. "1" or "1,255"
	TimeoutMs int    `json:"timeoutMs"`
	Identify  bool   `json:"identify"` // also read FC43/14 basic identification
}

// ScanState returns the current scan.
func (e *Engine) ScanState() ScanState {
	e.scan.mu.Lock()
	defer e.scan.mu.Unlock()
	return e.scan.copyLocked()
}

func (s *scanner) copyLocked() ScanState {
	st := s.st
	st.IDs = append([]IDResult{}, s.st.IDs...)
	st.Ranges = append([]MapRange{}, s.st.Ranges...)
	st.Unsupported = append([]int{}, s.st.Unsupported...)
	st.Hosts = append([]NetResult{}, s.st.Hosts...)
	return st
}

// StopScan cancels the running scan.
func (e *Engine) StopScan() {
	e.scan.mu.Lock()
	c := e.scan.cancel
	e.scan.mu.Unlock()
	if c != nil {
		c()
	}
}

// update changes the scan state and emits it, at most 10 times a second
// unless force is set.
func (e *Engine) scanUpdate(force bool, fn func(*ScanState)) {
	e.scan.emitMu.Lock()
	defer e.scan.emitMu.Unlock()
	e.scan.mu.Lock()
	fn(&e.scan.st)
	e.scan.st.Seq++
	if !force && time.Since(e.scan.lastEmit) < 100*time.Millisecond {
		e.scan.mu.Unlock()
		return
	}
	e.scan.lastEmit = time.Now()
	st := e.scan.copyLocked()
	e.scan.mu.Unlock()
	e.emit(EventScan, st)
}

func (e *Engine) startScan(kind string, total int, run func(ctx context.Context) error) error {
	e.scan.mu.Lock()
	if e.scan.st.Running {
		e.scan.mu.Unlock()
		return errors.New("a scan is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.scan.cancel = cancel
	e.scan.st = ScanState{Kind: kind, Running: true, Total: total, Started: time.Now().UnixMilli(), Seq: e.scan.st.Seq + 1}
	e.scan.mu.Unlock()
	e.scanUpdate(true, func(*ScanState) {})
	go func() {
		err := run(ctx)
		cancel()
		e.scanUpdate(true, func(s *ScanState) {
			s.Running, s.Finished, s.Phase = false, time.Now().UnixMilli(), ""
			if err != nil && !errors.Is(err, context.Canceled) {
				s.Error = err.Error()
			}
		})
	}()
	return nil
}

// Client returns a connection's client and its configured timeout.
func (e *Engine) Client(connID string) (*modbus.Client, time.Duration, error) {
	c := e.conn(connID)
	if c == nil {
		return nil, 0, errors.New("unknown connection")
	}
	return c.client, time.Duration(c.cfg.TimeoutMs) * time.Millisecond, nil
}

func (e *Engine) scanClient(connID string, timeoutMs int) (*modbus.Client, time.Duration, error) {
	cl, def, err := e.Client(connID)
	if err != nil {
		return nil, 0, err
	}
	if timeoutMs > 0 {
		def = time.Duration(timeoutMs) * time.Millisecond
	}
	if err := cl.Connect(); err != nil && !cl.IsOpen() {
		return nil, 0, err
	}
	return cl, def, nil
}

// probe reads one block. A dropped connection is retried once (the
// client reconnects), so a scan survives e.g. a restarted gateway.
func probe(cl *modbus.Client, unit byte, fc byte, address, quantity int, timeout time.Duration) (time.Duration, error) {
	_, _, d, err := cl.Read(unit, fc, address, quantity, timeout)
	if Classify(err) == OutcomeIO {
		_, _, d, err = cl.Read(unit, fc, address, quantity, timeout)
	}
	return d, err
}

// isGatewayMiss reports exceptions that mean "no such slave behind the gateway".
func isGatewayMiss(err error) bool {
	var ex *modbus.ExceptionError
	return errors.As(err, &ex) && (ex.Code == modbus.ExGatewayNoResponse || ex.Code == modbus.ExGatewayPathUnavail)
}

// StartIDScan probes slave IDs From..To with a one-value read.
func (e *Engine) StartIDScan(r IDScanRequest) error {
	if r.From < 1 || r.To > 247 || r.From > r.To {
		return errors.New("slave IDs must be within 1-247")
	}
	if r.Function < 1 || r.Function > 4 {
		r.Function = 3
	}
	cl, timeout, err := e.scanClient(r.ConnectionID, r.TimeoutMs)
	if err != nil {
		return err
	}
	return e.startScan(ScanIDs, r.To-r.From+1, func(ctx context.Context) error {
		e.scanUpdate(true, func(s *ScanState) { s.Connection = r.ConnectionID })
		for id := r.From; id <= r.To; id++ {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			d, err := probe(cl, byte(id), byte(r.Function), r.Address, 1, timeout)
			var res *IDResult
			var ex *modbus.ExceptionError
			switch {
			case err == nil:
				res = &IDResult{SlaveID: id, Status: "ok"}
			case isGatewayMiss(err):
			case errors.As(err, &ex):
				res = &IDResult{SlaveID: id, Status: "exception", Exception: ex.Error()}
			case Classify(err) == OutcomeCRC || Classify(err) == OutcomeInvalid:
				res = &IDResult{SlaveID: id, Status: "garbled", Exception: describeErr(err)}
			case Classify(err) == OutcomeIO:
				return err
			}
			if res != nil {
				res.Ms = float64(d.Microseconds()) / 1000
			}
			e.scanUpdate(res != nil, func(s *ScanState) {
				s.Done++
				if res != nil {
					s.IDs = append(s.IDs, *res)
				}
			})
		}
		return nil
	})
}

// Block sizes of the map scan, coarse to fine.
var mapSizes = []int{100, 10, 1}

// StartMapScan finds the readable addresses of one slave. Each function is
// read in blocks of 100; a rejected block is read again in blocks of 10,
// and the first address of each rejected block of 10 on its own. Blocks of
// 10 that are partly readable or touch readable addresses are then read
// address by address. A run shorter than 10 that contains no multiple of
// 10 and does not touch other readable addresses is missed.
func (e *Engine) StartMapScan(r MapScanRequest) error {
	if r.SlaveID < 0 || r.SlaveID > 255 {
		return errors.New("slave ID must be within 0-255")
	}
	if r.From < 0 || r.To > 0xFFFF || r.From > r.To {
		return errors.New("addresses must be within 0-65535")
	}
	if len(r.Functions) == 0 {
		r.Functions = []int{1, 2, 3, 4}
	}
	cl, timeout, err := e.scanClient(r.ConnectionID, r.TimeoutMs)
	if err != nil {
		return err
	}
	span := r.To - r.From + 1
	unit := byte(r.SlaveID)
	return e.startScan(ScanMap, span*len(r.Functions), func(ctx context.Context) error {
		e.scanUpdate(true, func(s *ScanState) { s.Connection, s.SlaveID = r.ConnectionID, r.SlaveID })
		for fi, fn := range r.Functions {
			fc := byte(fn)
			base := fi * span
			e.scanUpdate(true, func(s *ScanState) { s.Phase = modbus.FunctionName(fc); s.Done = base })
			// Is the function supported at all?
			if _, err := probe(cl, unit, fc, r.From, 1, timeout); err != nil {
				var ex *modbus.ExceptionError
				if errors.As(err, &ex) && ex.Code == modbus.ExIllegalFunction {
					e.scanUpdate(true, func(s *ScanState) { s.Unsupported = append(s.Unsupported, fn); s.Done = base + span })
					continue
				}
				if Classify(err) == OutcomeIO {
					return err
				}
				if fi == 0 && errors.Is(err, modbus.ErrTimeout) && !anyAnswer(cl, unit, timeout) {
					return fmt.Errorf("slave %d does not answer", r.SlaveID)
				}
			}
			valid, err := e.mapFunction(ctx, cl, unit, fc, r.From, r.To, timeout, func(done int) {
				e.scanUpdate(false, func(s *ScanState) { s.Done = base + done })
			})
			if err != nil {
				return err
			}
			ranges := toRanges(fn, valid, r.From)
			e.scanUpdate(true, func(s *ScanState) { s.Ranges = append(s.Ranges, ranges...); s.Done = base + span })
		}
		return nil
	})
}

// mapFunction returns which addresses from..to of one function are readable.
func (e *Engine) mapFunction(ctx context.Context, cl *modbus.Client, unit, fc byte, from, to int, timeout time.Duration, progress func(int)) ([]bool, error) {
	span := to - from + 1
	valid := make([]bool, span) // index = address - from
	isValid := func(a int) bool { return a >= from && a <= to && valid[a-from] }
	read := func(a, n int) (bool, error) {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		_, err := probe(cl, unit, fc, a, n, timeout)
		switch Classify(err) {
		case OutcomeOK:
			for i := a; i < a+n; i++ {
				valid[i-from] = true
			}
			return true, nil
		case OutcomeIO:
			return false, err
		}
		return false, nil
	}
	type block struct{ a, n int }
	split := func(b block, size int) []block {
		var out []block
		for a := b.a; a < b.a+b.n; a += size {
			out = append(out, block{a, min(size, b.a+b.n-a)})
		}
		return out
	}
	// Coarse pass, with every rejected block re-read in blocks of 10.
	var failed []block
	for _, b := range split(block{from, span}, mapSizes[0]) {
		ok, err := read(b.a, b.n)
		if err != nil {
			return nil, err
		}
		if !ok && b.n > mapSizes[1] {
			for _, c := range split(b, mapSizes[1]) {
				ok, err := read(c.a, c.n)
				if err != nil {
					return nil, err
				}
				if !ok {
					// Short runs such as coils 0-7: try the block's first address.
					if _, err := read(c.a, 1); err != nil {
						return nil, err
					}
					failed = append(failed, c)
				}
			}
		} else if !ok {
			failed = append(failed, b)
		}
		progress((b.a + b.n - from) * 8 / 10)
	}
	// Narrow down rejected blocks that touch readable addresses.
	for _, size := range mapSizes[2:] {
		var next []block
		for _, b := range failed {
			if b.n == 1 || !isValid(b.a-1) && !isValid(b.a+b.n) && !isValid(b.a) {
				continue
			}
			for _, c := range split(b, size) {
				if isValid(c.a) && c.n == 1 {
					continue
				}
				ok, err := read(c.a, c.n)
				if err != nil {
					return nil, err
				}
				if !ok {
					next = append(next, c)
				}
			}
		}
		failed = next
	}
	return valid, nil
}

// anyAnswer checks whether the slave answers anything at all (the first
// address may simply be unmapped on a slave that ignores bad addresses).
func anyAnswer(cl *modbus.Client, unit byte, timeout time.Duration) bool {
	_, err := cl.Diagnostic(unit, modbus.DiagReturnQueryData, 0xA55A, timeout)
	return err == nil || !errors.Is(err, modbus.ErrTimeout)
}

func toRanges(fn int, valid []bool, from int) []MapRange {
	var out []MapRange
	for i := 0; i < len(valid); i++ {
		if !valid[i] {
			continue
		}
		j := i
		for j+1 < len(valid) && valid[j+1] {
			j++
		}
		out = append(out, MapRange{Function: fn, From: from + i, To: from + j})
		i = j
	}
	return out
}

// ParseIPRange expands "192.168.1.1-254", "192.168.1.0/24",
// "10.0.0.5-10.0.1.20" or a single address (at most 4096 hosts).
func ParseIPRange(text string) ([]net.IP, error) {
	text = strings.TrimSpace(text)
	const limit = 4096
	var first, last uint32
	switch {
	case strings.Contains(text, "/"):
		_, n, err := net.ParseCIDR(text)
		if err != nil || n.IP.To4() == nil {
			return nil, fmt.Errorf("invalid network %q", text)
		}
		ones, _ := n.Mask.Size()
		first = binary.BigEndian.Uint32(n.IP.To4())
		last = first | (1<<(32-ones) - 1)
		if ones <= 30 { // skip network and broadcast addresses
			first++
			last--
		}
	case strings.Contains(text, "-"):
		a, b, _ := strings.Cut(text, "-")
		ip := net.ParseIP(strings.TrimSpace(a)).To4()
		if ip == nil {
			return nil, fmt.Errorf("invalid address %q", a)
		}
		first = binary.BigEndian.Uint32(ip)
		b = strings.TrimSpace(b)
		if n, err := strconv.Atoi(b); err == nil {
			if n < int(ip[3]) || n > 255 {
				return nil, fmt.Errorf("invalid range end %q", b)
			}
			last = first&0xFFFFFF00 | uint32(n)
		} else if ip2 := net.ParseIP(b).To4(); ip2 != nil {
			last = binary.BigEndian.Uint32(ip2)
		} else {
			return nil, fmt.Errorf("invalid address %q", b)
		}
	default:
		ip := net.ParseIP(text).To4()
		if ip == nil {
			return nil, fmt.Errorf("invalid address %q", text)
		}
		first = binary.BigEndian.Uint32(ip)
		last = first
	}
	if last < first {
		return nil, errors.New("the range ends before it starts")
	}
	if last-first >= limit {
		return nil, fmt.Errorf("the range has %d addresses; scan at most %d at a time", last-first+1, limit)
	}
	out := make([]net.IP, 0, last-first+1)
	for v := first; v <= last; v++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, v)
		out = append(out, ip)
	}
	return out, nil
}

func parseUnits(text string) ([]int, error) {
	if strings.TrimSpace(text) == "" {
		return []int{1}, nil
	}
	var out []int
	for _, f := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' }) {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 || n > 255 {
			return nil, fmt.Errorf("invalid unit ID %q", f)
		}
		out = append(out, n)
	}
	return out, nil
}

// StartNetworkScan connects to every address of the range and asks the
// open ones for a holding register on each unit ID.
func (e *Engine) StartNetworkScan(r NetworkScanRequest) error {
	hosts, err := ParseIPRange(r.Range)
	if err != nil {
		return err
	}
	units, err := parseUnits(r.Units)
	if err != nil {
		return err
	}
	if r.Port <= 0 || r.Port > 65535 {
		r.Port = 502
	}
	timeout := time.Duration(max(r.TimeoutMs, 50)) * time.Millisecond
	return e.startScan(ScanNetwork, len(hosts), func(ctx context.Context) error {
		jobs := make(chan net.IP)
		var wg sync.WaitGroup
		for w := 0; w < min(64, len(hosts)); w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for ip := range jobs {
					res := e.probeHost(ctx, ip.String(), r.Port, units, timeout, r.Identify)
					e.scanUpdate(res != nil, func(s *ScanState) {
						s.Done++
						if res != nil {
							s.Hosts = append(s.Hosts, *res)
							sort.Slice(s.Hosts, func(i, j int) bool { return ipLess(s.Hosts[i].Host, s.Hosts[j].Host) })
						}
					})
				}
			}()
		}
		for _, ip := range hosts {
			select {
			case jobs <- ip:
			case <-ctx.Done():
				close(jobs)
				wg.Wait()
				return ctx.Err()
			}
		}
		close(jobs)
		wg.Wait()
		return nil
	})
}

func ipLess(a, b string) bool {
	x, y := net.ParseIP(a).To4(), net.ParseIP(b).To4()
	if x == nil || y == nil {
		return a < b
	}
	return binary.BigEndian.Uint32(x) < binary.BigEndian.Uint32(y)
}

func (e *Engine) probeHost(ctx context.Context, host string, port int, units []int, timeout time.Duration, identify bool) *NetResult {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()
	d := net.Dialer{Timeout: timeout}
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil
	}
	c.Close()
	res := &NetResult{Host: host, Port: port, Status: "open", Units: []int{}, Ms: float64(time.Since(start).Microseconds()) / 1000}
	tr, err := modbus.NewTransport(modbus.Config{Mode: modbus.ModeTCP, Host: host, Port: port, ConnectTimeout: max(timeout*4, time.Second)}, e.traffic.observer("Network scan"))
	if err != nil {
		return res
	}
	cl := modbus.NewClient(tr)
	defer cl.Close()
	reqTimeout := max(timeout*4, 300*time.Millisecond)
	for _, u := range units {
		if ctx.Err() != nil {
			return res
		}
		_, err := cl.ReadRegisters(byte(u), modbus.FuncReadHoldingRegisters, 0, 1, reqTimeout)
		var ex *modbus.ExceptionError
		switch {
		case err == nil:
		case isGatewayMiss(err):
			continue
		case errors.As(err, &ex):
			res.Exception = ex.Error()
		default:
			continue
		}
		res.Status = "modbus"
		res.Units = append(res.Units, u)
		if identify && len(res.Identity) == 0 {
			if id, err := cl.ReadDeviceIdentification(byte(u), modbus.DeviceIDBasic, reqTimeout); err == nil {
				for _, o := range id.Objects {
					res.Identity = append(res.Identity, o.Value)
				}
			}
		}
	}
	return res
}
