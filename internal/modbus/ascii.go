package modbus

import (
	"encoding/hex"
	"errors"
	"net"
	"strings"
	"time"
)

// LRC is the Modbus ASCII checksum: the two's complement of the byte sum.
func LRC(b []byte) byte {
	var s byte
	for _, x := range b {
		s += x
	}
	return -s
}

// EncodeASCII builds a Modbus ASCII frame: ':' hex(unit, PDU, LRC) CR LF.
func EncodeASCII(unit byte, p PDU) []byte {
	raw := append([]byte{unit}, p.Bytes()...)
	raw = append(raw, LRC(raw))
	out := make([]byte, 0, 3+2*len(raw))
	out = append(out, ':')
	out = append(out, strings.ToUpper(hex.EncodeToString(raw))...)
	return append(out, '\r', '\n')
}

// DecodeASCII validates an ASCII frame and splits it.
func DecodeASCII(frame []byte) (unit byte, p PDU, err error) {
	s := string(frame)
	if !strings.HasPrefix(s, ":") || !strings.HasSuffix(s, "\r\n") {
		return 0, PDU{}, invalid("ASCII frame must start with ':' and end with CR LF")
	}
	raw, err := hex.DecodeString(s[1 : len(s)-2])
	if err != nil {
		return 0, PDU{}, invalid("ASCII frame is not hexadecimal")
	}
	if len(raw) < 3 {
		return 0, PDU{}, invalid("ASCII frame too short")
	}
	n := len(raw) - 1
	if want := LRC(raw[:n]); raw[n] != want {
		return 0, PDU{}, invalid("LRC error (got %02X, want %02X)", raw[n], want)
	}
	return raw[0], PDU{Function: raw[1], Data: append([]byte(nil), raw[2:n]...)}, nil
}

// maxASCIIFrame is ':' + 2×(1+253+1) hex characters + CR LF.
const maxASCIIFrame = 1 + 2*255 + 2

// readASCIIFrame reads one frame from s: bytes before ':' are dropped and
// a new ':' restarts the frame, as the spec requires.
func readASCIIFrame(s stream, deadline time.Time) ([]byte, error) {
	frame := make([]byte, 0, 64)
	buf := make([]byte, 1)
	for {
		n, err := s.readUntil(buf, deadline)
		if err != nil {
			return frame, err
		}
		if n == 0 {
			continue
		}
		c := buf[0]
		switch {
		case c == ':':
			frame = append(frame[:0], c)
		case len(frame) == 0:
			// noise between frames
		default:
			frame = append(frame, c)
			if c == '\n' {
				return frame, nil
			}
			if len(frame) > maxASCIIFrame {
				return frame, invalid("ASCII frame too long")
			}
		}
	}
}

// ---- Modbus ASCII over a stream (serial or TCP) ----

type asciiTransport struct {
	dial    func() (stream, error)
	s       stream
	dirty   bool
	traffic TrafficFunc
}

func (t *asciiTransport) Open() error {
	if t.s != nil {
		return nil
	}
	s, err := t.dial()
	if err != nil {
		return err
	}
	t.s, t.dirty = s, true
	return nil
}

func (t *asciiTransport) Close() error {
	if t.s == nil {
		return nil
	}
	err := t.s.Close()
	t.s = nil
	return err
}

func (t *asciiTransport) IsOpen() bool { return t.s != nil }

func (t *asciiTransport) Exchange(unit byte, req PDU, timeout time.Duration) (PDU, error) {
	if t.s == nil {
		return PDU{}, errors.New("not connected")
	}
	if t.dirty {
		t.s.drain()
		t.dirty = false
	}
	adu := EncodeASCII(unit, req)
	t.traffic(Tx, adu)
	if _, err := t.s.Write(adu); err != nil {
		return PDU{}, err
	}
	if unit == 0 {
		return PDU{}, errBroadcast
	}
	frame, err := readASCIIFrame(t.s, time.Now().Add(timeout))
	if len(frame) > 0 {
		t.traffic(Rx, frame)
	}
	if err != nil {
		t.dirty = true
		if errors.Is(err, ErrTimeout) && len(frame) > 0 {
			return PDU{}, invalid("incomplete ASCII frame (%d characters)", len(frame))
		}
		return PDU{}, err
	}
	u, p, err := DecodeASCII(frame)
	if err != nil {
		return PDU{}, err
	}
	if u != unit {
		return PDU{}, invalid("response from slave %d, expected %d", u, unit)
	}
	return p, nil
}

// ---- Modbus UDP (MBAP header in each datagram) ----

type mbapUDPTransport struct {
	addr    string
	c       net.Conn
	txID    uint16
	traffic TrafficFunc
}

func (t *mbapUDPTransport) Open() error {
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

func (t *mbapUDPTransport) Close() error {
	if t.c == nil {
		return nil
	}
	err := t.c.Close()
	t.c = nil
	return err
}

func (t *mbapUDPTransport) IsOpen() bool { return t.c != nil }

func (t *mbapUDPTransport) Exchange(unit byte, req PDU, timeout time.Duration) (PDU, error) {
	if t.c == nil {
		return PDU{}, errors.New("not connected")
	}
	t.txID++
	adu := EncodeMBAP(t.txID, unit, req)
	t.traffic(Tx, adu)
	if _, err := t.c.Write(adu); err != nil {
		return PDU{}, err
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
			return PDU{}, err // e.g. ICMP port unreachable: nobody is listening
		}
		t.traffic(Rx, buf[:n])
		if n < MBAPHeaderLen+1 {
			continue
		}
		h, err := DecodeMBAPHeader(buf[:MBAPHeaderLen])
		if err != nil || MBAPHeaderLen+int(h.Length)-1 != n {
			return PDU{}, invalid("malformed Modbus UDP datagram")
		}
		if h.TxID != t.txID {
			continue // late answer to an earlier request
		}
		if h.Unit != unit {
			return PDU{}, invalid("response from unit %d, expected %d", h.Unit, unit)
		}
		return PDU{Function: buf[MBAPHeaderLen], Data: append([]byte(nil), buf[MBAPHeaderLen+1:n]...)}, nil
	}
}
