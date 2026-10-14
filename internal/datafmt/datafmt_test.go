package datafmt

import "testing"

func TestKnownFloat(t *testing.T) {
	// 123.456f = 0x42F6E979
	cases := map[Format][]uint16{
		"float_abcd": {0x42F6, 0xE979},
		"float_cdab": {0xE979, 0x42F6},
		"float_badc": {0xF642, 0x79E9},
		"float_dcba": {0x79E9, 0xF642},
	}
	for f, regs := range cases {
		if got := FormatValue(f, regs); got != "123.456" {
			t.Errorf("%s: got %s", f, got)
		}
		enc, err := ParseValue(f, "123.456")
		if err != nil || enc[0] != regs[0] || enc[1] != regs[1] {
			t.Errorf("%s: encode got %04X %v", f, enc, err)
		}
	}
}

func Test16Bit(t *testing.T) {
	if v := FormatValue(Signed, []uint16{0xFFFF}); v != "-1" {
		t.Errorf("signed %s", v)
	}
	if v := FormatValue(Unsigned, []uint16{0xFFFF}); v != "65535" {
		t.Errorf("unsigned %s", v)
	}
	if v := FormatValue(Hex, []uint16{0x00AB}); v != "0x00AB" {
		t.Errorf("hex %s", v)
	}
	if v := FormatValue(Binary, []uint16{0x8001}); v != "1000 0000 0000 0001" {
		t.Errorf("binary %s", v)
	}
	if r, err := ParseValue(Hex, "0xbeef"); err != nil || r[0] != 0xBEEF {
		t.Errorf("parse hex %v %v", r, err)
	}
	if _, err := ParseValue(Signed, "40000"); err == nil {
		t.Error("40000 must not fit signed 16-bit")
	}
}

func TestRoundTripAllFormats(t *testing.T) {
	samples := map[string]string{
		"signed": "-1234", "unsigned": "54321", "hex": "0x1A2B", "binary": "1010 0000 1111 0101",
		"long": "-123456789", "ulong": "4000000000", "float": "-0.15625",
		"long64": "-9000000000000000000", "double": "3.141592653589793",
	}
	for _, info := range All() {
		kind, _ := split(info.ID)
		in := samples[kind]
		regs, err := ParseValue(info.ID, in)
		if err != nil {
			t.Fatalf("%s parse: %v", info.ID, err)
		}
		if len(regs) != info.Registers {
			t.Fatalf("%s: %d regs, want %d", info.ID, len(regs), info.Registers)
		}
		if out := FormatValue(info.ID, regs); out != in {
			t.Errorf("%s: %q -> %q", info.ID, in, out)
		}
	}
}

func TestDoubleOrders(t *testing.T) {
	// 1.0 = 0x3FF0000000000000
	be := []uint16{0x3FF0, 0, 0, 0}
	if v := FormatValue("double_abcd", be); v != "1" {
		t.Errorf("abcd %s", v)
	}
	if v := FormatValue("double_cdab", []uint16{0, 0, 0, 0x3FF0}); v != "1" {
		t.Errorf("cdab %s", v)
	}
	if v := FormatValue("double_dcba", []uint16{0, 0, 0, 0xF03F}); v != "1" {
		t.Errorf("dcba %s", v)
	}
}
