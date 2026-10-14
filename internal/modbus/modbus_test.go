package modbus

import (
	"bytes"
	"errors"
	"testing"
)

func TestCRC16KnownVector(t *testing.T) {
	// Read 10 holding registers from slave 1 at address 0.
	adu := EncodeRTU(1, PDU{Function: 3, Data: []byte{0, 0, 0, 0x0A}})
	want := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A, 0xC5, 0xCD}
	if !bytes.Equal(adu, want) {
		t.Fatalf("got % X, want % X", adu, want)
	}
	u, p, err := DecodeRTU(adu)
	if err != nil || u != 1 || p.Function != 3 {
		t.Fatalf("decode: %v %d %v", err, u, p)
	}
	adu[7] ^= 0xFF
	if _, _, err := DecodeRTU(adu); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("expected CRC error, got %v", err)
	}
}

func TestMBAPEncoding(t *testing.T) {
	got := EncodeMBAP(0x1234, 0x11, PDU{Function: 3, Data: []byte{0, 0x6B, 0, 3}})
	want := []byte{0x12, 0x34, 0, 0, 0, 6, 0x11, 3, 0, 0x6B, 0, 3}
	if !bytes.Equal(got, want) {
		t.Fatalf("got % X, want % X", got, want)
	}
	h, err := DecodeMBAPHeader(got)
	if err != nil || h.TxID != 0x1234 || h.Length != 6 || h.Unit != 0x11 {
		t.Fatalf("header %+v %v", h, err)
	}
}

func TestRequestLimits(t *testing.T) {
	if _, err := ReadRequest(FuncReadHoldingRegisters, 0, 126); err == nil {
		t.Error("126 registers must be rejected")
	}
	if _, err := ReadRequest(FuncReadCoils, 65535, 2); err == nil {
		t.Error("range past 65535 must be rejected")
	}
	if _, err := ReadRequest(FuncWriteSingleCoil, 0, 1); err == nil {
		t.Error("FC05 is not a read")
	}
}

func TestCheckResponse(t *testing.T) {
	req, _ := ReadRequest(FuncReadHoldingRegisters, 0, 2)
	if err := CheckResponse(req, PDU{Function: 3, Data: []byte{4, 0, 1, 0, 2}}); err != nil {
		t.Fatal(err)
	}
	var ex *ExceptionError
	if err := CheckResponse(req, PDU{Function: 0x83, Data: []byte{2}}); !errors.As(err, &ex) || ex.Code != 2 {
		t.Fatalf("expected exception 02, got %v", err)
	}
	if err := CheckResponse(req, PDU{Function: 3, Data: []byte{2, 0, 1}}); err == nil {
		t.Fatal("short response must fail")
	}
}

func TestBitPacking(t *testing.T) {
	bits := []bool{true, false, true, true, false, false, true, true, true, false}
	packed := PackBits(bits)
	if !bytes.Equal(packed, []byte{0xCD, 0x01}) {
		t.Fatalf("packed % X", packed)
	}
	got := UnpackBits(packed, len(bits))
	for i := range bits {
		if got[i] != bits[i] {
			t.Fatalf("bit %d mismatch", i)
		}
	}
}

func TestResponseLengths(t *testing.T) {
	cases := []struct {
		head []byte
		want int
	}{
		{[]byte{1, 3, 20}, 25},
		{[]byte{1, 0x83, 2}, 5},
		{[]byte{1, 6, 0}, 8},
		{[]byte{1, 0x2B, 0}, 0},
	}
	for _, c := range cases {
		if got := RTUResponseLength(c.head); got != c.want {
			t.Errorf("% X: got %d want %d", c.head, got, c.want)
		}
	}
	if l, _ := RTURequestLength([]byte{1, 0x10, 0, 0, 0, 2, 4}); l != 13 {
		t.Errorf("FC16 request length %d", l)
	}
}
