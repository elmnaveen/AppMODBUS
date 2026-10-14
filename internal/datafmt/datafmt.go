// Package datafmt converts between raw Modbus registers and the display
// formats offered by AppMODBUS (16/32/64-bit integers and IEEE floats in
// every common byte order).
package datafmt

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Format identifies a display format. 32- and 64-bit formats carry their
// byte order as a suffix, e.g. "float_cdab".
type Format string

const (
	Signed   Format = "signed"
	Unsigned Format = "unsigned"
	Hex      Format = "hex"
	Binary   Format = "binary"
)

// Order is a byte order. ABCD is big endian (Modbus standard).
type Order string

const (
	ABCD Order = "abcd" // big endian
	CDAB Order = "cdab" // word swapped
	BADC Order = "badc" // byte swapped
	DCBA Order = "dcba" // little endian
)

var orders = []Order{ABCD, CDAB, BADC, DCBA}

// Info describes a format for the UI.
type Info struct {
	ID        Format `json:"id"`
	Label     string `json:"label"`
	Registers int    `json:"registers"`
	Group     string `json:"group"`
}

func orderLabel(o Order, regs int) string {
	if regs == 4 {
		switch o {
		case ABCD:
			return "AB CD EF GH"
		case CDAB:
			return "GH EF CD AB"
		case BADC:
			return "BA DC FE HG"
		case DCBA:
			return "HG FE DC BA"
		}
	}
	s := strings.ToUpper(string(o))
	return s[:2] + " " + s[2:]
}

// All lists every supported format in menu order.
func All() []Info {
	out := []Info{
		{Signed, "Signed", 1, "16-bit"},
		{Unsigned, "Unsigned", 1, "16-bit"},
		{Hex, "Hex", 1, "16-bit"},
		{Binary, "Binary", 1, "16-bit"},
	}
	add := func(kind, label string, regs int, group string) {
		for _, o := range orders {
			out = append(out, Info{Format(kind + "_" + string(o)), label + " " + orderLabel(o, regs), regs, group})
		}
	}
	add("long", "Long", 2, "32-bit")
	add("ulong", "Unsigned Long", 2, "32-bit")
	add("float", "Float", 2, "32-bit")
	add("long64", "Long64", 4, "64-bit")
	add("double", "Double", 4, "64-bit")
	return out
}

var byID = func() map[Format]Info {
	m := map[Format]Info{}
	for _, i := range All() {
		m[i.ID] = i
	}
	return m
}()

// Valid reports whether f is a known format.
func Valid(f Format) bool { _, ok := byID[f]; return ok }

// Registers returns how many 16-bit registers a value of format f spans.
func Registers(f Format) int {
	if i, ok := byID[f]; ok {
		return i.Registers
	}
	return 1
}

func split(f Format) (kind string, o Order) {
	s := string(f)
	if i := strings.LastIndexByte(s, '_'); i >= 0 {
		return s[:i], Order(s[i+1:])
	}
	return s, ABCD
}

// reorder converts between wire order and big endian; it is its own inverse.
func reorder(b []byte, o Order) {
	switch o {
	case CDAB: // reverse word order
		for i, j := 0, len(b)-2; i < j; i, j = i+2, j-2 {
			b[i], b[j] = b[j], b[i]
			b[i+1], b[j+1] = b[j+1], b[i+1]
		}
	case BADC: // swap bytes within each word
		for i := 0; i+1 < len(b); i += 2 {
			b[i], b[i+1] = b[i+1], b[i]
		}
	case DCBA: // reverse all bytes
		for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
			b[i], b[j] = b[j], b[i]
		}
	}
}

func regsToBE(regs []uint16, o Order) []byte {
	b := make([]byte, 2*len(regs))
	for i, r := range regs {
		binary.BigEndian.PutUint16(b[2*i:], r)
	}
	reorder(b, o)
	return b
}

func beToRegs(b []byte, o Order) []uint16 {
	reorder(b, o)
	out := make([]uint16, len(b)/2)
	for i := range out {
		out[i] = binary.BigEndian.Uint16(b[2*i:])
	}
	return out
}

// FormatValue renders the registers of one value. regs must hold exactly
// Registers(f) registers.
func FormatValue(f Format, regs []uint16) string {
	if len(regs) < Registers(f) || len(regs) == 0 {
		return "-"
	}
	kind, o := split(f)
	switch kind {
	case "signed":
		return strconv.Itoa(int(int16(regs[0])))
	case "unsigned":
		return strconv.Itoa(int(regs[0]))
	case "hex":
		return fmt.Sprintf("0x%04X", regs[0])
	case "binary":
		s := fmt.Sprintf("%016b", regs[0])
		return s[0:4] + " " + s[4:8] + " " + s[8:12] + " " + s[12:16]
	}
	b := regsToBE(regs[:Registers(f)], o)
	switch kind {
	case "long":
		return strconv.FormatInt(int64(int32(binary.BigEndian.Uint32(b))), 10)
	case "ulong":
		return strconv.FormatUint(uint64(binary.BigEndian.Uint32(b)), 10)
	case "float":
		return formatFloat(float64(math.Float32frombits(binary.BigEndian.Uint32(b))), 32)
	case "long64":
		return strconv.FormatInt(int64(binary.BigEndian.Uint64(b)), 10)
	case "double":
		return formatFloat(math.Float64frombits(binary.BigEndian.Uint64(b)), 64)
	}
	return "?"
}

func formatFloat(v float64, bits int) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "+Inf"
	case math.IsInf(v, -1):
		return "-Inf"
	}
	return strconv.FormatFloat(v, 'g', -1, bits)
}

// ParseValue converts user input into the registers for format f.
func ParseValue(f Format, text string) ([]uint16, error) {
	s := strings.TrimSpace(text)
	if s == "" {
		return nil, fmt.Errorf("empty value")
	}
	kind, o := split(f)
	switch kind {
	case "signed":
		v, err := strconv.ParseInt(s, 10, 16)
		if err != nil {
			return nil, rangeErr(s, "-32768..32767")
		}
		return []uint16{uint16(int16(v))}, nil
	case "unsigned":
		v, err := strconv.ParseUint(s, 10, 16)
		if err != nil {
			return nil, rangeErr(s, "0..65535")
		}
		return []uint16{uint16(v)}, nil
	case "hex":
		s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
		v, err := strconv.ParseUint(s, 16, 16)
		if err != nil {
			return nil, rangeErr(text, "0x0000..0xFFFF")
		}
		return []uint16{uint16(v)}, nil
	case "binary":
		s = strings.NewReplacer(" ", "", "_", "").Replace(strings.TrimPrefix(s, "0b"))
		v, err := strconv.ParseUint(s, 2, 16)
		if err != nil {
			return nil, rangeErr(text, "16 binary digits")
		}
		return []uint16{uint16(v)}, nil
	}
	b := make([]byte, 2*Registers(f))
	switch kind {
	case "long":
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return nil, rangeErr(s, "-2147483648..2147483647")
		}
		binary.BigEndian.PutUint32(b, uint32(int32(v)))
	case "ulong":
		v, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return nil, rangeErr(s, "0..4294967295")
		}
		binary.BigEndian.PutUint32(b, uint32(v))
	case "float":
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return nil, fmt.Errorf("%q is not a valid float", s)
		}
		binary.BigEndian.PutUint32(b, math.Float32bits(float32(v)))
	case "long64":
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, rangeErr(s, "a signed 64-bit integer")
		}
		binary.BigEndian.PutUint64(b, uint64(v))
	case "double":
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a valid double", s)
		}
		binary.BigEndian.PutUint64(b, math.Float64bits(v))
	default:
		return nil, fmt.Errorf("unknown format %q", f)
	}
	return beToRegs(b, o), nil
}

func rangeErr(s, want string) error {
	return fmt.Errorf("%q is out of range (expected %s)", s, want)
}

// IsInteger reports whether f holds an integer value.
func IsInteger(f Format) bool {
	kind, _ := split(f)
	return kind != "float" && kind != "double"
}

// Numeric returns the value of regs in format f as a float64. Hex and
// binary are treated as unsigned 16-bit.
func Numeric(f Format, regs []uint16) (float64, bool) {
	if len(regs) < Registers(f) || len(regs) == 0 {
		return 0, false
	}
	kind, o := split(f)
	switch kind {
	case "signed":
		return float64(int16(regs[0])), true
	case "unsigned", "hex", "binary":
		return float64(regs[0]), true
	}
	b := regsToBE(regs[:Registers(f)], o)
	switch kind {
	case "long":
		return float64(int32(binary.BigEndian.Uint32(b))), true
	case "ulong":
		return float64(binary.BigEndian.Uint32(b)), true
	case "float":
		v := float64(math.Float32frombits(binary.BigEndian.Uint32(b)))
		return v, !math.IsNaN(v) && !math.IsInf(v, 0)
	case "long64":
		return float64(int64(binary.BigEndian.Uint64(b))), true
	case "double":
		v := math.Float64frombits(binary.BigEndian.Uint64(b))
		return v, !math.IsNaN(v) && !math.IsInf(v, 0)
	}
	return 0, false
}

var intRanges = map[string][2]float64{
	"signed": {math.MinInt16, math.MaxInt16}, "unsigned": {0, math.MaxUint16},
	"hex": {0, math.MaxUint16}, "binary": {0, math.MaxUint16},
	"long": {math.MinInt32, math.MaxInt32}, "ulong": {0, math.MaxUint32},
	"long64": {math.MinInt64, math.MaxInt64},
}

// FromNumeric encodes v in format f, rounding to the nearest integer for
// integer formats and rejecting values out of range.
func FromNumeric(f Format, v float64) ([]uint16, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil, fmt.Errorf("value is not a finite number")
	}
	kind, _ := split(f)
	if !IsInteger(f) {
		return ParseValue(f, strconv.FormatFloat(v, 'g', -1, 64))
	}
	r := math.Round(v)
	lim := intRanges[kind]
	lo, hi := lim[0], lim[1]
	if r < lo || r > hi {
		return nil, fmt.Errorf("raw value %s is out of range for %s", strconv.FormatFloat(r, 'f', -1, 64), f)
	}
	switch kind {
	case "hex", "binary", "unsigned":
		return ParseValue(Unsigned, strconv.FormatFloat(r, 'f', 0, 64))
	}
	return ParseValue(f, strconv.FormatFloat(r, 'f', 0, 64))
}
