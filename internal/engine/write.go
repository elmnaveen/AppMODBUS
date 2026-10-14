package engine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"appmodbus/internal/datafmt"
	"appmodbus/internal/modbus"
)

// WriteRequest is a manual write from the Write dialogs.
type WriteRequest struct {
	ConnectionID string `json:"connectionId"`
	SlaveID      int    `json:"slaveId"`
	Function     int    `json:"function"` // 5, 6, 15 or 16
	Address      int    `json:"address"`
	Values       []int  `json:"values"` // registers, or 0/1 for coils
}

// Write performs a manual write and then refreshes polls on the connection.
func (e *Engine) Write(w WriteRequest) error {
	c := e.conn(w.ConnectionID)
	if c == nil {
		return errors.New("unknown connection")
	}
	if len(w.Values) == 0 {
		return errors.New("no values to write")
	}
	var (
		req modbus.PDU
		err error
	)
	switch byte(w.Function) {
	case modbus.FuncWriteSingleCoil:
		req, err = modbus.WriteSingleCoilRequest(w.Address, w.Values[0] != 0)
	case modbus.FuncWriteSingleRegister:
		if w.Values[0] < -32768 || w.Values[0] > 65535 {
			return fmt.Errorf("value %d does not fit a register", w.Values[0])
		}
		req, err = modbus.WriteSingleRegisterRequest(w.Address, uint16(w.Values[0]))
	case modbus.FuncWriteMultipleCoils:
		bits := make([]bool, len(w.Values))
		for i, v := range w.Values {
			bits[i] = v != 0
		}
		req, err = modbus.WriteMultipleCoilsRequest(w.Address, bits)
	case modbus.FuncWriteMultipleRegisters:
		regs := make([]uint16, len(w.Values))
		for i, v := range w.Values {
			if v < -32768 || v > 65535 {
				return fmt.Errorf("value %d does not fit a register", v)
			}
			regs[i] = uint16(v)
		}
		req, err = modbus.WriteMultipleRegistersRequest(w.Address, regs)
	default:
		return fmt.Errorf("function %d is not a write function", w.Function)
	}
	if err != nil {
		return err
	}
	if w.SlaveID < 0 || w.SlaveID > 255 {
		return errors.New("slave ID must be 0..255")
	}
	_, err = c.client.Do(byte(w.SlaveID), req, time.Duration(c.cfg.TimeoutMs)*time.Millisecond)
	e.setConnErr(c, err)
	if err != nil {
		return errors.New(describeErr(err))
	}
	e.refresh(w.ConnectionID)
	return nil
}

// WriteCell writes a value typed into a poll window cell, encoding it with
// the cell's display format. forceMultiple selects FC15/FC16 even for a
// single coil/register (some devices only accept those).
func (e *Engine) WriteCell(pollID string, address int, text string, forceMultiple bool) error {
	p := e.poll(pollID)
	if p == nil {
		return errors.New("unknown poll")
	}
	p.mu.Lock()
	cfg := p.cfg
	f := cfg.Format
	if cf, ok := cfg.CellFormats[address]; ok {
		f = cf
	}
	tag, tagged := cfg.Tags[address]
	p.mu.Unlock()

	w := WriteRequest{ConnectionID: cfg.ConnectionID, SlaveID: cfg.SlaveID, Address: address}
	switch byte(cfg.Function) {
	case modbus.FuncReadCoils:
		on, err := parseBool(text)
		if err != nil {
			return err
		}
		w.Function = int(modbus.FuncWriteSingleCoil)
		if forceMultiple {
			w.Function = int(modbus.FuncWriteMultipleCoils)
		}
		w.Values = []int{b2i(on)}
	case modbus.FuncReadHoldingRegisters:
		var regs []uint16
		var err error
		if tagged && tag.Scales() {
			// The user types the engineering value; convert back to raw.
			v, perr := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), tag.Unit)), 64)
			if perr != nil {
				return fmt.Errorf("%q is not a number", text)
			}
			regs, err = datafmt.FromNumeric(f, (v-tag.Offset)/tag.Gain)
		} else {
			regs, err = datafmt.ParseValue(f, text)
		}
		if err != nil {
			return err
		}
		w.Function = int(modbus.FuncWriteMultipleRegisters)
		if len(regs) == 1 && !forceMultiple {
			w.Function = int(modbus.FuncWriteSingleRegister)
		}
		for _, r := range regs {
			w.Values = append(w.Values, int(r))
		}
	case modbus.FuncReadDiscreteInputs:
		return errors.New("discrete inputs are read-only")
	default:
		return errors.New("input registers are read-only")
	}
	return e.Write(w)
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "on", "true":
		return true, nil
	case "0", "off", "false":
		return false, nil
	}
	return false, fmt.Errorf("%q is not a coil value (use 0/1 or on/off)", s)
}

// refresh makes running polls on a connection read again immediately.
func (e *Engine) refresh(connID string) {
	for _, p := range e.pollsOn(connID) {
		p.mu.Lock()
		if p.kick != nil {
			select {
			case p.kick <- struct{}{}:
			default:
			}
		}
		p.mu.Unlock()
	}
}

// SetTag sets (or, with nil, removes) the tag of the register at address.
func (e *Engine) SetTag(pollID string, address int, t *Tag) error {
	p := e.poll(pollID)
	if p == nil {
		return errors.New("unknown poll")
	}
	if t != nil {
		if err := normalizeTag(t); err != nil {
			return err
		}
	}
	p.mu.Lock()
	if t == nil || *t == (Tag{Gain: 1, Decimals: -1}) { // empty tag
		delete(p.cfg.Tags, address)
	} else {
		p.cfg.Tags[address] = *t
	}
	p.rebuildCells()
	p.mu.Unlock()
	e.publish(p)
	e.changed()
	return nil
}
