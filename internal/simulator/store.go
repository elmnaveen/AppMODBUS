// Package simulator implements a Modbus slave used to test AppMODBUS
// without hardware.
package simulator

import (
	"encoding/binary"
	"sync"

	"appmodbus/internal/modbus"
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
		if addr+qty > 0x10000 {
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
		s.mu.Lock()
		s.coils[u16(0)] = v == 0xFF00
		s.mu.Unlock()
		return modbus.PDU{Function: fc, Data: append([]byte(nil), d...)}

	case modbus.FuncWriteSingleRegister:
		if len(d) != 4 {
			return exception(fc, modbus.ExIllegalDataValue)
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
		if addr+qty > 0x10000 {
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
	}
	return exception(fc, modbus.ExIllegalFunction)
}
