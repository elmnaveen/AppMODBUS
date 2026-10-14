package modbus

import "testing"

func TestASCIIFrame(t *testing.T) {
	req, _ := ReadRequest(FuncReadHoldingRegisters, 0, 1)
	f := EncodeASCII(1, req)
	if string(f) != ":010300000001FB\r\n" {
		t.Fatalf("frame %q", f)
	}
	u, p, err := DecodeASCII(f)
	if err != nil || u != 1 || p.Function != 3 || len(p.Data) != 4 {
		t.Fatalf("decode %d %+v %v", u, p, err)
	}
	bad := []byte(":010300000001FA\r\n")
	if _, _, err := DecodeASCII(bad); err == nil {
		t.Fatal("expected an LRC error")
	}
	if _, _, err := DecodeASCII([]byte(":01XX\r\n")); err == nil {
		t.Fatal("expected a hex error")
	}
}

func TestReadWriteRegistersRequest(t *testing.T) {
	p, err := ReadWriteRegistersRequest(3, 6, 14, []uint16{0x00FF, 0x00FF, 0x00FF})
	if err != nil {
		t.Fatal(err)
	}
	// Example from the Modbus spec (6.17).
	want := []byte{0x17, 0x00, 0x03, 0x00, 0x06, 0x00, 0x0E, 0x00, 0x03, 0x06, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF}
	if string(p.Bytes()) != string(want) {
		t.Fatalf("% X", p.Bytes())
	}
	if _, err := ReadWriteRegistersRequest(0, 126, 0, []uint16{1}); err == nil {
		t.Fatal("expected a range error")
	}
	if n, need := RTURequestLength(EncodeRTU(1, p)[:11]); n != 13+6 || need != 11 {
		t.Fatalf("request length %d need %d", n, need)
	}
}
