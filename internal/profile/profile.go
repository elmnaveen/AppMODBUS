// Package profile implements device profiles: reusable register maps
// (address, function, name, format, scaling, unit) that can be applied to
// any slave to create ready-labelled poll windows.
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"appmodbus/internal/datafmt"
	"appmodbus/internal/engine"
	"appmodbus/internal/modbus"
)

// File identification.
const (
	FormatID       = "appmodbus-profile"
	CurrentVersion = 1
)

// Register is one entry of a register map.
type Register struct {
	Address  int            `json:"address"`  // protocol address (0-based)
	Function int            `json:"function"` // 1, 2, 3 or 4
	Name     string         `json:"name"`
	Format   datafmt.Format `json:"format"` // "bit" for coils/discrete inputs
	Gain     float64        `json:"gain"`
	Offset   float64        `json:"offset"`
	Unit     string         `json:"unit"`
	Decimals int            `json:"decimals"` // -1 = automatic
	Note     string         `json:"note,omitempty"`
}

// UnmarshalJSON applies defaults for fields missing from hand-written files.
func (r *Register) UnmarshalJSON(b []byte) error {
	type plain Register
	p := plain{Function: 3, Format: datafmt.Unsigned, Gain: 1, Decimals: -1}
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*r = Register(p)
	return nil
}

// Span is the number of registers (or bits) the value occupies.
func (r Register) Span() int {
	if modbus.IsBitFunction(byte(r.Function)) {
		return 1
	}
	return datafmt.Registers(r.Format)
}

// Profile is a device register map.
type Profile struct {
	Format         string     `json:"format"`
	Version        int        `json:"version"`
	Name           string     `json:"name"`
	Manufacturer   string     `json:"manufacturer"`
	Model          string     `json:"model"`
	Description    string     `json:"description"`
	DefaultSlaveID int        `json:"defaultSlaveId"`
	Registers      []Register `json:"registers"`
}

// Normalize validates a profile, fixing defaults in place.
func (p *Profile) Normalize() error {
	p.Format, p.Version = FormatID, CurrentVersion
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = strings.TrimSpace(p.Manufacturer + " " + p.Model)
	}
	if p.Name == "" {
		return errors.New("the profile needs a name")
	}
	if p.DefaultSlaveID < 1 || p.DefaultSlaveID > 247 {
		p.DefaultSlaveID = 1
	}
	if len(p.Registers) == 0 {
		return errors.New("the profile has no registers")
	}
	seen := map[[2]int]string{}
	for i := range p.Registers {
		r := &p.Registers[i]
		if r.Function < 1 || r.Function > 4 {
			return fmt.Errorf("%s: function must be 1-4", r.label())
		}
		if modbus.IsBitFunction(byte(r.Function)) {
			r.Format = "bit"
		} else if !datafmt.Valid(r.Format) {
			return fmt.Errorf("%s: unknown format %q", r.label(), r.Format)
		}
		if r.Address < 0 || r.Address+r.Span() > 0x10000 {
			return fmt.Errorf("%s: address out of range", r.label())
		}
		if r.Gain == 0 {
			r.Gain = 1
		}
		if r.Decimals < -1 || r.Decimals > 9 {
			r.Decimals = -1
		}
		r.Name, r.Unit = strings.TrimSpace(r.Name), strings.TrimSpace(r.Unit)
		key := [2]int{r.Function, r.Address}
		if other, dup := seen[key]; dup {
			return fmt.Errorf("%s: address %d is also used by %q", r.label(), r.Address, other)
		}
		seen[key] = r.Name
	}
	sort.SliceStable(p.Registers, func(i, j int) bool {
		a, b := p.Registers[i], p.Registers[j]
		if a.Function != b.Function {
			return fnOrder(a.Function) < fnOrder(b.Function)
		}
		return a.Address < b.Address
	})
	return nil
}

func (r Register) label() string {
	if r.Name != "" {
		return fmt.Sprintf("register %q", r.Name)
	}
	return fmt.Sprintf("register at %d", r.Address)
}

// fnOrder lists holding/input registers before bits in generated windows.
func fnOrder(fc int) int { return map[int]int{3: 0, 4: 1, 1: 2, 2: 3}[fc] }

// Encode serialises a profile.
func Encode(p Profile) ([]byte, error) {
	if err := p.Normalize(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(p, "", "  ")
}

// Decode parses and validates a profile file.
func Decode(b []byte) (Profile, error) {
	var p Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return Profile{}, fmt.Errorf("not a profile file: %w", err)
	}
	if p.Format != FormatID {
		return Profile{}, errors.New("not an AppMODBUS device profile")
	}
	if p.Version > CurrentVersion {
		return Profile{}, fmt.Errorf("profile format %d needs a newer AppMODBUS", p.Version)
	}
	return p, p.Normalize()
}

// Gap limits: registers closer than this are read in one request.
const (
	maxGapRegisters = 8
	maxGapBits      = 32
)

var fnShort = map[int]string{1: "Coils", 2: "Inputs", 3: "Holding", 4: "Input regs"}

// ToPolls turns a profile into poll window definitions for one slave,
// grouping nearby registers into as few requests as the protocol allows.
func ToPolls(p Profile, connectionID string, slaveID, scanRateMs int) ([]engine.PollConfig, error) {
	if err := p.Normalize(); err != nil {
		return nil, err
	}
	title := p.Model
	if title == "" {
		title = p.Name
	}
	var out []engine.PollConfig
	var group []Register
	flush := func() {
		if len(group) == 0 {
			return
		}
		first, last := group[0], group[len(group)-1]
		end := last.Address + last.Span()
		cfg := engine.PollConfig{
			Name:         fmt.Sprintf("%s #%d · %s %d–%d", title, slaveID, fnShort[first.Function], first.Address, end-1),
			ConnectionID: connectionID, SlaveID: slaveID, Function: first.Function,
			Address: first.Address, Quantity: end - first.Address, ScanRateMs: scanRateMs,
			Format: mostCommonFormat(group), CellFormats: map[int]datafmt.Format{}, Tags: map[int]engine.Tag{},
		}
		for _, r := range group {
			if !modbus.IsBitFunction(byte(r.Function)) && r.Format != cfg.Format {
				cfg.CellFormats[r.Address] = r.Format
			}
			cfg.Tags[r.Address] = engine.Tag{Name: r.Name, Gain: r.Gain, Offset: r.Offset, Unit: r.Unit, Decimals: r.Decimals, Note: r.Note}
		}
		out = append(out, cfg)
		group = nil
	}
	for _, r := range p.Registers {
		if len(group) > 0 {
			first, last := group[0], group[len(group)-1]
			bits := modbus.IsBitFunction(byte(r.Function))
			max, gap := modbus.MaxReadRegisters, maxGapRegisters
			if bits {
				max, gap = modbus.MaxReadBits, maxGapBits
			}
			// Continuation registers of the previous value must not start a value.
			lastEnd := last.Address + last.Span()
			if r.Function != first.Function || r.Address-lastEnd > gap || r.Address+r.Span()-first.Address > max {
				flush()
			}
		}
		group = append(group, r)
	}
	flush()
	return out, nil
}

func mostCommonFormat(rs []Register) datafmt.Format {
	if modbus.IsBitFunction(byte(rs[0].Function)) {
		return datafmt.Signed
	}
	count := map[datafmt.Format]int{}
	best := rs[0].Format
	for _, r := range rs {
		count[r.Format]++
		if count[r.Format] > count[best] {
			best = r.Format
		}
	}
	return best
}

// FromPoll builds a profile from a poll window: every displayed value with
// its format and tag.
func FromPoll(poll engine.PollConfig, name string) Profile {
	p := Profile{Name: name, DefaultSlaveID: poll.SlaveID}
	bits := modbus.IsBitFunction(byte(poll.Function))
	for a := poll.Address; a < poll.Address+poll.Quantity; {
		f := poll.Format
		if cf, ok := poll.CellFormats[a]; ok {
			f = cf
		}
		r := Register{Address: a, Function: poll.Function, Format: f, Gain: 1, Decimals: -1}
		if bits {
			r.Format = "bit"
		}
		if t, ok := poll.Tags[a]; ok {
			r.Name, r.Gain, r.Offset, r.Unit, r.Decimals, r.Note = t.Name, t.Gain, t.Offset, t.Unit, t.Decimals, t.Note
		}
		span := 1
		if !bits {
			span = datafmt.Registers(f)
		}
		if a+span > poll.Address+poll.Quantity {
			break
		}
		p.Registers = append(p.Registers, r)
		a += span
	}
	return p
}
