package modbus

import (
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

// Mode selects the physical transport and framing.
type Mode string

const (
	ModeRTU        Mode = "rtu"     // RTU over a serial port
	ModeTCP        Mode = "tcp"     // Modbus TCP/IP (MBAP)
	ModeRTUOverTCP Mode = "rtu-tcp" // RTU frames tunnelled over a TCP socket
	ModeRTUOverUDP Mode = "rtu-udp" // RTU frames carried in UDP datagrams
)

// Config describes how to reach a slave.
type Config struct {
	Mode Mode

	// Serial settings (ModeRTU).
	SerialPort string
	BaudRate   int
	DataBits   int
	Parity     string // "N", "E" or "O"
	StopBits   int

	// Network settings (TCP/UDP modes).
	Host string
	Port int

	ConnectTimeout time.Duration
}

// Describe returns a short human readable summary of the configuration.
func (c Config) Describe() string {
	switch c.Mode {
	case ModeRTU:
		return fmt.Sprintf("RTU %s %d-%d%s%d", c.SerialPort, c.BaudRate, c.DataBits, c.Parity, c.StopBits)
	case ModeTCP:
		return fmt.Sprintf("TCP %s", c.address())
	case ModeRTUOverTCP:
		return fmt.Sprintf("RTU/TCP %s", c.address())
	case ModeRTUOverUDP:
		return fmt.Sprintf("RTU/UDP %s", c.address())
	}
	return string(c.Mode)
}

func (c Config) address() string { return net.JoinHostPort(c.Host, strconv.Itoa(c.Port)) }

// Direction of a traffic frame.
type Direction string

const (
	Tx Direction = "Tx"
	Rx Direction = "Rx"
)

// TrafficFunc observes every raw frame sent or received.
type TrafficFunc func(dir Direction, frame []byte)

// ErrTimeout is returned when the slave does not answer in time.
var ErrTimeout = errors.New("timeout error")

// errBroadcast marks a successful broadcast write (no response expected).
var errBroadcast = errors.New("broadcast")

// Transport sends one request and waits for the matching response.
// Implementations are not safe for concurrent use; Client serialises access.
type Transport interface {
	Open() error
	Close() error
	IsOpen() bool
	Exchange(unit byte, req PDU, timeout time.Duration) (PDU, error)
}

// NewTransport creates a transport for cfg. traffic may be nil.
func NewTransport(cfg Config, traffic TrafficFunc) (Transport, error) {
	if traffic == nil {
		traffic = func(Direction, []byte) {}
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 3 * time.Second
	}
	switch cfg.Mode {
	case ModeRTU:
		if cfg.SerialPort == "" {
			return nil, errors.New("no serial port selected")
		}
		if cfg.DataBits == 0 {
			cfg.DataBits = 8
		}
		if cfg.StopBits == 0 {
			cfg.StopBits = 1
		}
		if cfg.Parity == "" {
			cfg.Parity = "N"
		}
		t35 := SerialFrameTimes(cfg.BaudRate, cfg.DataBits, cfg.Parity, cfg.StopBits)
		return &rtuTransport{
			dial:    func() (stream, error) { return openSerial(cfg) },
			silence: time.Duration(t35 * float64(time.Millisecond)),
			traffic: traffic,
		}, nil
	case ModeTCP, ModeRTUOverTCP, ModeRTUOverUDP:
		if cfg.Host == "" || cfg.Port <= 0 || cfg.Port > 65535 {
			return nil, errors.New("invalid host or port")
		}
	default:
		return nil, fmt.Errorf("unknown mode %q", cfg.Mode)
	}
	addr := cfg.address()
	switch cfg.Mode {
	case ModeTCP:
		return &tcpTransport{addr: addr, timeout: cfg.ConnectTimeout, traffic: traffic}, nil
	case ModeRTUOverTCP:
		return &rtuTransport{
			dial: func() (stream, error) {
				c, err := net.DialTimeout("tcp", addr, cfg.ConnectTimeout)
				if err != nil {
					return nil, err
				}
				return &netStream{c}, nil
			},
			traffic: traffic,
		}, nil
	default:
		return &udpTransport{addr: addr, traffic: traffic}, nil
	}
}

// stream abstracts a byte stream with deadline based reads.
type stream interface {
	io.Writer
	io.Closer
	// readUntil reads into p, returning ErrTimeout if nothing arrives by d.
	readUntil(p []byte, d time.Time) (int, error)
	// drain discards any stale input.
	drain()
}

func readFull(s stream, p []byte, d time.Time) (int, error) {
	n := 0
	for n < len(p) {
		m, err := s.readUntil(p[n:], d)
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// ---- serial ----

type serialStream struct{ p serial.Port }

func openSerial(cfg Config) (stream, error) {
	mode := &serial.Mode{BaudRate: cfg.BaudRate, DataBits: cfg.DataBits}
	switch cfg.Parity {
	case "E":
		mode.Parity = serial.EvenParity
	case "O":
		mode.Parity = serial.OddParity
	default:
		mode.Parity = serial.NoParity
	}
	if cfg.StopBits == 2 {
		mode.StopBits = serial.TwoStopBits
	} else {
		mode.StopBits = serial.OneStopBit
	}
	p, err := serial.Open(cfg.SerialPort, mode)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w%s", cfg.SerialPort, err, SerialPermissionHint(err))
	}
	return &serialStream{p}, nil
}

func (s *serialStream) Write(b []byte) (int, error) { return s.p.Write(b) }
func (s *serialStream) Close() error                { return s.p.Close() }
func (s *serialStream) drain()                      { _ = s.p.ResetInputBuffer() }

func (s *serialStream) readUntil(b []byte, d time.Time) (int, error) {
	t := time.Until(d)
	if t <= 0 {
		return 0, ErrTimeout
	}
	if err := s.p.SetReadTimeout(t); err != nil {
		return 0, err
	}
	n, err := s.p.Read(b)
	if err != nil {
		return n, err
	}
	if n == 0 {
		return 0, ErrTimeout
	}
	return n, nil
}

// SerialPermissionHint explains how to get access to serial ports on Linux
// when opening one was refused.
func SerialPermissionHint(err error) string {
	if runtime.GOOS != "linux" || err == nil || !strings.Contains(strings.ToLower(err.Error()), "permission") {
		return ""
	}
	return " (add your user to the dialout group: sudo usermod -aG dialout $USER, then log out and in again)"
}

// ListSerialPorts returns the serial ports present on this machine.
func ListSerialPorts() ([]string, error) { return serial.GetPortsList() }

// ---- TCP stream ----

type netStream struct{ c net.Conn }

func (s *netStream) Write(b []byte) (int, error) { return s.c.Write(b) }
func (s *netStream) Close() error                { return s.c.Close() }

func (s *netStream) readUntil(b []byte, d time.Time) (int, error) {
	_ = s.c.SetReadDeadline(d)
	n, err := s.c.Read(b)
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return n, ErrTimeout
	}
	return n, err
}

func (s *netStream) drain() {
	buf := make([]byte, 256)
	for {
		_ = s.c.SetReadDeadline(time.Now().Add(2 * time.Millisecond))
		if n, err := s.c.Read(buf); n == 0 || err != nil {
			return
		}
	}
}

// ---- RTU over a stream (serial or TCP) ----

type rtuTransport struct {
	dial    func() (stream, error)
	s       stream
	silence time.Duration
	last    time.Time
	dirty   bool
	traffic TrafficFunc
}

func (t *rtuTransport) Open() error {
	if t.s != nil {
		return nil
	}
	s, err := t.dial()
	if err != nil {
		return err
	}
	t.s = s
	t.dirty = true
	return nil
}

func (t *rtuTransport) Close() error {
	if t.s == nil {
		return nil
	}
	err := t.s.Close()
	t.s = nil
	return err
}

func (t *rtuTransport) IsOpen() bool { return t.s != nil }

func (t *rtuTransport) Exchange(unit byte, req PDU, timeout time.Duration) (PDU, error) {
	if t.s == nil {
		return PDU{}, errors.New("not connected")
	}
	if wait := t.silence - time.Since(t.last); wait > 0 {
		time.Sleep(wait)
	}
	if t.dirty {
		t.s.drain()
		t.dirty = false
	}
	adu := EncodeRTU(unit, req)
	t.traffic(Tx, adu)
	if _, err := t.s.Write(adu); err != nil {
		return PDU{}, err
	}
	defer func() { t.last = time.Now() }()
	if unit == 0 {
		return PDU{}, errBroadcast
	}

	deadline := time.Now().Add(timeout)
	buf := make([]byte, 260)
	n, err := readFull(t.s, buf[:3], deadline)
	if err != nil {
		t.dirty = true
		if n > 0 {
			t.traffic(Rx, buf[:n])
		}
		return PDU{}, err
	}
	total := RTUResponseLength(buf[:3])
	if total == 0 || total > len(buf) {
		// Unknown layout: collect what arrives, then report it.
		m, _ := t.s.readUntil(buf[3:], time.Now().Add(50*time.Millisecond))
		t.dirty = true
		t.traffic(Rx, buf[:3+m])
		return PDU{}, invalid("unexpected function code %02X", buf[1])
	}
	m, err := readFull(t.s, buf[3:total], deadline)
	frame := buf[:3+m]
	t.traffic(Rx, frame)
	if err != nil {
		t.dirty = true
		if errors.Is(err, ErrTimeout) {
			return PDU{}, invalid("incomplete frame (%d of %d bytes)", len(frame), total)
		}
		return PDU{}, err
	}
	u, p, err := DecodeRTU(frame)
	if err != nil {
		t.dirty = true
		return PDU{}, err
	}
	if u != unit {
		return PDU{}, invalid("response from slave %d, expected %d", u, unit)
	}
	return p, nil
}

// ---- Modbus TCP (MBAP) ----

type tcpTransport struct {
	addr    string
	timeout time.Duration
	c       net.Conn
	txID    uint16
	traffic TrafficFunc
}

func (t *tcpTransport) Open() error {
	if t.c != nil {
		return nil
	}
	c, err := net.DialTimeout("tcp", t.addr, t.timeout)
	if err != nil {
		return err
	}
	t.c = c
	return nil
}

func (t *tcpTransport) Close() error {
	if t.c == nil {
		return nil
	}
	err := t.c.Close()
	t.c = nil
	return err
}

func (t *tcpTransport) IsOpen() bool { return t.c != nil }

func (t *tcpTransport) Exchange(unit byte, req PDU, timeout time.Duration) (PDU, error) {
	if t.c == nil {
		return PDU{}, errors.New("not connected")
	}
	t.txID++
	adu := EncodeMBAP(t.txID, unit, req)
	t.traffic(Tx, adu)
	_ = t.c.SetWriteDeadline(time.Now().Add(timeout))
	if _, err := t.c.Write(adu); err != nil {
		return PDU{}, err
	}
	s := &netStream{t.c}
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 260)
	for {
		n, err := readFull(s, buf[:MBAPHeaderLen], deadline)
		if err != nil {
			if n > 0 {
				t.traffic(Rx, buf[:n])
				_ = t.Close() // stream is out of sync
			}
			return PDU{}, err
		}
		h, err := DecodeMBAPHeader(buf[:MBAPHeaderLen])
		if err != nil {
			t.traffic(Rx, buf[:MBAPHeaderLen])
			_ = t.Close()
			return PDU{}, err
		}
		end := MBAPHeaderLen + int(h.Length) - 1
		m, err := readFull(s, buf[MBAPHeaderLen:end], deadline)
		t.traffic(Rx, buf[:MBAPHeaderLen+m])
		if err != nil {
			_ = t.Close()
			return PDU{}, err
		}
		if h.TxID != t.txID {
			continue // late answer to an earlier, timed out request
		}
		if h.Unit != unit {
			return PDU{}, invalid("response from unit %d, expected %d", h.Unit, unit)
		}
		data := make([]byte, end-MBAPHeaderLen-1)
		copy(data, buf[MBAPHeaderLen+1:end])
		return PDU{Function: buf[MBAPHeaderLen], Data: data}, nil
	}
}

// ---- RTU over UDP ----

type udpTransport struct {
	addr    string
	c       net.Conn
	traffic TrafficFunc
}

func (t *udpTransport) Open() error {
	if t.c != nil {
		return nil
	}
	c, err := net.Dial("udp", t.addr)
	if err != nil {
		return err
	}
	t.c = c
	return nil
}

func (t *udpTransport) Close() error {
	if t.c == nil {
		return nil
	}
	err := t.c.Close()
	t.c = nil
	return err
}

func (t *udpTransport) IsOpen() bool { return t.c != nil }

func (t *udpTransport) Exchange(unit byte, req PDU, timeout time.Duration) (PDU, error) {
	if t.c == nil {
		return PDU{}, errors.New("not connected")
	}
	adu := EncodeRTU(unit, req)
	t.traffic(Tx, adu)
	if _, err := t.c.Write(adu); err != nil {
		return PDU{}, err
	}
	if unit == 0 {
		return PDU{}, errBroadcast
	}
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 512)
	for {
		_ = t.c.SetReadDeadline(deadline)
		n, err := t.c.Read(buf)
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return PDU{}, ErrTimeout
		}
		if err != nil {
			// e.g. ICMP port unreachable: nobody is listening.
			return PDU{}, err
		}
		t.traffic(Rx, buf[:n])
		u, p, err := DecodeRTU(buf[:n])
		if err != nil {
			return PDU{}, err
		}
		if u != unit {
			continue
		}
		return p, nil
	}
}

// ---- Client ----

// Client serialises requests over one transport, so several pollers can
// share a single serial line or socket without colliding.
type Client struct {
	mu   sync.Mutex
	t    Transport
	last time.Time

	// Gap is the minimum idle time between the end of one transaction and
	// the start of the next ("delay between polls").
	Gap time.Duration
}

// NewClient wraps a transport.
func NewClient(t Transport) *Client { return &Client{t: t} }

// Connect opens the underlying transport.
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t.Open()
}

// Close closes the underlying transport.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t.Close()
}

// IsOpen reports whether the transport is open.
func (c *Client) IsOpen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t.IsOpen()
}

// Do sends req to unit and validates the response. For broadcast requests
// (unit 0) it returns a zero PDU and no error.
func (c *Client) Do(unit byte, req PDU, timeout time.Duration) (PDU, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.t.IsOpen() {
		if err := c.t.Open(); err != nil {
			return PDU{}, err
		}
	}
	if wait := c.Gap - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	resp, err := c.t.Exchange(unit, req, timeout)
	c.last = time.Now()
	if errors.Is(err, errBroadcast) {
		return PDU{}, nil
	}
	if err != nil {
		if !errors.Is(err, ErrTimeout) && !errors.Is(err, ErrInvalidResponse) {
			_ = c.t.Close() // I/O failure: reconnect on next request
		}
		return PDU{}, err
	}
	return resp, CheckResponse(req, resp)
}

// ReadBits performs FC01/FC02.
func (c *Client) ReadBits(unit, fc byte, address, quantity int, timeout time.Duration) ([]bool, error) {
	req, err := ReadRequest(fc, address, quantity)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(unit, req, timeout)
	if err != nil {
		return nil, err
	}
	if unit == 0 {
		return nil, errors.New("broadcast reads get no response")
	}
	return DecodeBits(resp, quantity), nil
}

// ReadRegisters performs FC03/FC04.
func (c *Client) ReadRegisters(unit, fc byte, address, quantity int, timeout time.Duration) ([]uint16, error) {
	req, err := ReadRequest(fc, address, quantity)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(unit, req, timeout)
	if err != nil {
		return nil, err
	}
	if unit == 0 {
		return nil, errors.New("broadcast reads get no response")
	}
	return DecodeRegisters(resp), nil
}
