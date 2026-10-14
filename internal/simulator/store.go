// Package simulator implements a Modbus slave used to test Modbus Studio
// without hardware.
package simulator

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"modbusstudio/internal/modbus"
)

// Table identifies one of the four Modbus data tables.
type Table string

const (
	Coils            Table = "coils"
	DiscreteInputs   Table = "discrete"
	HoldingRegisters Table = "holding"
	InputRegisters   Table = "input"
)

// Store holds the full 65536-entry address space of every table.
type Store struct {
	mu       sync.RWMutex
	coils    []bool
	discrete []bool
	holding  []uint16
	input    []uint16
	// limits restricts the addresses a table answers on (nil = all), so the
	// simulator can stand in for a device with a sparse register map.
	limits map[Table][]Span
}

// Span is an inclusive address range.
type Span struct{ From, To int }

// ParseRanges reads "0-99, 1000-1019, 2000" into spans. Blank means all.
func ParseRanges(text string) ([]Span, error) {
	var out []Span
	for _, part := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		a, b, isRange := strings.Cut(part, "-")
		from, err := strconv.Atoi(a)
		if err != nil {
			return nil, fmt.Errorf("invalid address %q", part)
		}
		to := from
		if isRange {
			if to, err = strconv.Atoi(b); err != nil {
				return nil, fmt.Errorf("invalid address range %q", part)
			}
		}
		if from < 0 || to > 0xFFFF || from > to {
			return nil, fmt.Errorf("address range %q must be within 0-65535", part)
		}
		out = append(out, Span{from, to})
	}
	return out, nil
}

// SetLimits sets the valid address ranges per table ("" = whole table).
func (s *Store) SetLimits(limits map[Table]string) error {
	parsed := map[Table][]Span{}
	for t, text := range limits {
		spans, err := ParseRanges(text)
		if err != nil {
			return fmt.Errorf("%s: %w", t, err)
		}
		if len(spans) > 0 {
			parsed[t] = spans
		}
	}
	s.mu.Lock()
	s.limits = parsed
	s.mu.Unlock()
	return nil
}

// allowed reports whether every address of the range is valid. Callers
// hold s.mu.
func (s *Store) allowed(t Table, address, quantity int) bool {
	spans, ok := s.limits[t]
	if !ok {
		return true
	}
	end := address + quantity - 1
	for _, sp := range spans {
		if address >= sp.From && end <= sp.To {
			return true
		}
	}
	return false
}

func (s *Store) checkLimits(t Table, address, quantity int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.allowed(t, address, quantity)
}

var tableOf = map[byte]Table{
	modbus.FuncReadCoils: Coils, modbus.FuncReadDiscreteInputs: DiscreteInputs,
	modbus.FuncReadHoldingRegisters: HoldingRegisters, modbus.FuncReadInputRegisters: InputRegisters,
	modbus.FuncWriteSingleCoil: Coils, modbus.FuncWriteMultipleCoils: Coils,
	modbus.FuncWriteSingleRegister: HoldingRegisters, modbus.FuncWriteMultipleRegisters: HoldingRegisters,
}

// NewStore returns an all-zero data store.
func NewStore() *Store {
	return &Store{
		coils:    make([]bool, 0x10000),
		discrete: make([]bool, 0x10000),
		holding:  make([]uint16, 0x10000),
		input:    make([]uint16, 0x10000),
	}
}

func clampRange(address, quantity int) (int, int) {
	if address < 0 {
		address = 0
	}
	if address > 0xFFFF {
		address = 0xFFFF
	}
	if quantity < 0 {
		quantity = 0
	}
	if address+quantity > 0x10000 {
		quantity = 0x10000 - address
	}
	return address, quantity
}

// Read returns values of a table as ints (bits are 0/1).
func (s *Store) Read(t Table, address, quantity int) []int {
	address, quantity = clampRange(address, quantity)
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int, quantity)
	for i := range out {
		switch t {
		case Coils:
			out[i] = b2i(s.coils[address+i])
		case DiscreteInputs:
			out[i] = b2i(s.discrete[address+i])
		case HoldingRegisters:
			out[i] = int(s.holding[address+i])
		case InputRegisters:
			out[i] = int(s.input[address+i])
		}
	}
	return out
}

// Write sets values in a table (any table, including read-only ones,
// can be edited from the simulator UI).
func (s *Store) Write(t Table, address int, values []int) {
	address, n := clampRange(address, len(values))
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i < n; i++ {
		v := values[i]
		switch t {
		case Coils:
			s.coils[address+i] = v != 0
		case DiscreteInputs:
			s.discrete[address+i] = v != 0
		case HoldingRegisters:
			s.holding[address+i] = uint16(v)
		case InputRegisters:
			s.input[address+i] = uint16(v)
		}
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func exception(fc, code byte) modbus.PDU {
	return modbus.PDU{Function: fc | 0x80, Data: []byte{code}}
}

// Handle processes a request PDU and returns the response PDU.
func (s *Store) Handle(req modbus.PDU) modbus.PDU {
	d := req.Data
	u16 := func(i int) int { return int(binary.BigEndian.Uint16(d[i:])) }
	fc := req.Function
	switch fc {
	case modbus.FuncReadCoils, modbus.FuncReadDiscreteInputs,
		modbus.FuncReadHoldingRegisters, modbus.FuncReadInputRegisters:
		if len(d) != 4 {
			return exception(fc, modbus.ExIllegalDataValue)
		}
		addr, qty := u16(0), u16(2)
		max := modbus.MaxReadRegisters
		if modbus.IsBitFunction(fc) {
			max = modbus.MaxReadBits
		}
		if qty < 1 || qty > max {
			return exception(fc, modbus.ExIllegalDataValue)
		}
		if addr+qty > 0x10000 || !s.checkLimits(tableOf[fc], addr, qty) {
			return exception(fc, modbus.ExIllegalDataAddress)
		}
		s.mu.RLock()
		defer s.mu.RUnlock()
		switch fc {
		case modbus.FuncReadCoils, modbus.FuncReadDiscreteInputs:
			src := s.coils
			if fc == modbus.FuncReadDiscreteInputs {
				src = s.discrete
			}
			packed := modbus.PackBits(src[addr : addr+qty])
			return modbus.PDU{Function: fc, Data: append([]byte{byte(len(packed))}, packed...)}
		default:
			src := s.holding
			if fc == modbus.FuncReadInputRegisters {
				src = s.input
			}
			out := make([]byte, 1+2*qty)
			out[0] = byte(2 * qty)
			for i := 0; i < qty; i++ {
				binary.BigEndian.PutUint16(out[1+2*i:], src[addr+i])
			}
			return modbus.PDU{Function: fc, Data: out}
		}

	case modbus.FuncWriteSingleCoil:
		if len(d) != 4 {
			return exception(fc, modbus.ExIllegalDataValue)
		}
		v := u16(2)
		if v != 0xFF00 && v != 0x0000 {
			return exception(fc, modbus.ExIllegalDataValue)
		}
		if !s.checkLimits(Coils, u16(0), 1) {
			return exception(fc, modbus.ExIllegalDataAddress)
		}
		s.mu.Lock()
		s.coils[u16(0)] = v == 0xFF00
		s.mu.Unlock()
		return modbus.PDU{Function: fc, Data: append([]byte(nil), d...)}

	case modbus.FuncWriteSingleRegister:
		if len(d) != 4 {
			return exception(fc, modbus.ExIllegalDataValue)
		}
		if !s.checkLimits(HoldingRegisters, u16(0), 1) {
			return exception(fc, modbus.ExIllegalDataAddress)
		}
		s.mu.Lock()
		s.holding[u16(0)] = uint16(u16(2))
		s.mu.Unlock()
		return modbus.PDU{Function: fc, Data: append([]byte(nil), d...)}

	case modbus.FuncWriteMultipleCoils, modbus.FuncWriteMultipleRegisters:
		if len(d) < 5 {
			return exception(fc, modbus.ExIllegalDataValue)
		}
		addr, qty, count := u16(0), u16(2), int(d[4])
		isBits := fc == modbus.FuncWriteMultipleCoils
		max, want := modbus.MaxWriteRegisters, 2*qty
		if isBits {
			max, want = modbus.MaxWriteBits, (qty+7)/8
		}
		if qty < 1 || qty > max || count != want || len(d) != 5+count {
			return exception(fc, modbus.ExIllegalDataValue)
		}
		if addr+qty > 0x10000 || !s.checkLimits(tableOf[fc], addr, qty) {
			return exception(fc, modbus.ExIllegalDataAddress)
		}
		s.mu.Lock()
		if isBits {
			copy(s.coils[addr:], modbus.UnpackBits(d[5:], qty))
		} else {
			for i := 0; i < qty; i++ {
				s.holding[addr+i] = binary.BigEndian.Uint16(d[5+2*i:])
			}
		}
		s.mu.Unlock()
		return modbus.PDU{Function: fc, Data: append([]byte(nil), d[:4]...)}

	case modbus.FuncReadWriteRegisters:
		if len(d) < 9 {
			return exception(fc, modbus.ExIllegalDataValue)
		}
		ra, rq, wa, wq, count := u16(0), u16(2), u16(4), u16(6), int(d[8])
		if rq < 1 || rq > modbus.MaxReadRegisters || wq < 1 || wq > modbus.MaxRWWrite || count != 2*wq || len(d) != 9+count {
			return exception(fc, modbus.ExIllegalDataValue)
		}
		if ra+rq > 0x10000 || wa+wq > 0x10000 || !s.checkLimits(HoldingRegisters, ra, rq) || !s.checkLimits(HoldingRegisters, wa, wq) {
			return exception(fc, modbus.ExIllegalDataAddress)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		// The write is performed before the read (spec 6.17).
		for i := 0; i < wq; i++ {
			s.holding[wa+i] = binary.BigEndian.Uint16(d[9+2*i:])
		}
		out := make([]byte, 1+2*rq)
		out[0] = byte(2 * rq)
		for i := 0; i < rq; i++ {
			binary.BigEndian.PutUint16(out[1+2*i:], s.holding[ra+i])
		}
		return modbus.PDU{Function: fc, Data: out}
	}
	return exception(fc, modbus.ExIllegalFunction)
}

// Block is a run of consecutive values starting at Start.
type Block struct {
	Start  int   `json:"start"`
	Values []int `json:"values"`
}

// Snapshot returns the non-zero contents of every table as compact blocks.
// Runs of up to 8 zeros inside a block are kept to avoid fragmenting it.
func (s *Store) Snapshot() map[Table][]Block {
	out := map[Table][]Block{}
	for _, t := range []Table{Coils, DiscreteInputs, HoldingRegisters, InputRegisters} {
		vals := s.Read(t, 0, 0x10000)
		var blocks []Block
		for i := 0; i < len(vals); {
			if vals[i] == 0 {
				i++
				continue
			}
			start, end, zeros := i, i, 0
			for j := i; j < len(vals) && zeros <= 8; j++ {
				if vals[j] == 0 {
					zeros++
				} else {
					zeros, end = 0, j
				}
			}
			blocks = append(blocks, Block{Start: start, Values: append([]int(nil), vals[start:end+1]...)})
			i = end + 1
		}
		if len(blocks) > 0 {
			out[t] = blocks
		}
	}
	return out
}

// Restore clears the store and writes the given blocks.
func (s *Store) Restore(data map[Table][]Block) {
	s.mu.Lock()
	clear(s.coils)
	clear(s.discrete)
	clear(s.holding)
	clear(s.input)
	s.mu.Unlock()
	for t, blocks := range data {
		for _, b := range blocks {
			s.Write(t, b.Start, b.Values)
		}
	}
}
