// Package modbus implements the Modbus application protocol (PDU encoding)
// and the RTU / TCP (MBAP) framings used by AppMODBUS.
package modbus

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Function codes supported by AppMODBUS.
const (
	FuncReadCoils              byte = 0x01
	FuncReadDiscreteInputs     byte = 0x02
	FuncReadHoldingRegisters   byte = 0x03
	FuncReadInputRegisters     byte = 0x04
	FuncWriteSingleCoil        byte = 0x05
	FuncWriteSingleRegister    byte = 0x06
	FuncWriteMultipleCoils     byte = 0x0F
	FuncWriteMultipleRegisters byte = 0x10
	FuncReadWriteRegisters     byte = 0x17
)

// Protocol limits from the Modbus Application Protocol spec v1.1b3.
const (
	MaxReadBits       = 2000
	MaxReadRegisters  = 125
	MaxWriteBits      = 1968
	MaxWriteRegisters = 123
	MaxRWWrite        = 121 // write part of FC23
)

// Exception codes.
const (
	ExIllegalFunction    byte = 0x01
	ExIllegalDataAddress byte = 0x02
	ExIllegalDataValue   byte = 0x03
	ExServerDeviceFail   byte = 0x04
	ExAcknowledge        byte = 0x05
	ExServerDeviceBusy   byte = 0x06
	ExGatewayPathUnavail byte = 0x0A
	ExGatewayNoResponse  byte = 0x0B
)

// PDU is a Modbus protocol data unit: function code plus data.
type PDU struct {
	Function byte
	Data     []byte
}

// Bytes returns the wire representation of the PDU.
func (p PDU) Bytes() []byte {
	b := make([]byte, 1+len(p.Data))
	b[0] = p.Function
	copy(b[1:], p.Data)
	return b
}

// ExceptionError is returned when a slave answers with an exception response.
type ExceptionError struct {
	Function byte
	Code     byte
}

func (e *ExceptionError) Error() string {
	return fmt.Sprintf("Modbus exception %02X: %s", e.Code, ExceptionText(e.Code))
}

// ExceptionText returns a human readable name of an exception code.
func ExceptionText(code byte) string {
	switch code {
	case ExIllegalFunction:
		return "Illegal function"
	case ExIllegalDataAddress:
		return "Illegal data address"
	case ExIllegalDataValue:
		return "Illegal data value"
	case ExServerDeviceFail:
		return "Slave device failure"
	case ExAcknowledge:
		return "Acknowledge"
	case ExServerDeviceBusy:
		return "Slave device busy"
	case ExGatewayPathUnavail:
		return "Gateway path unavailable"
	case ExGatewayNoResponse:
		return "Gateway target device failed to respond"
	}
	return "Unknown exception"
}

// ErrInvalidResponse is wrapped by all malformed-response errors.
var ErrInvalidResponse = errors.New("invalid response")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidResponse, fmt.Sprintf(format, args...))
}

// FunctionName returns a short description of a function code.
func FunctionName(fc byte) string {
	switch fc {
	case FuncReadCoils:
		return "Read Coils"
	case FuncReadDiscreteInputs:
		return "Read Discrete Inputs"
	case FuncReadHoldingRegisters:
		return "Read Holding Registers"
	case FuncReadInputRegisters:
		return "Read Input Registers"
	case FuncWriteSingleCoil:
		return "Write Single Coil"
	case FuncWriteSingleRegister:
		return "Write Single Register"
	case FuncWriteMultipleCoils:
		return "Write Multiple Coils"
	case FuncWriteMultipleRegisters:
		return "Write Multiple Registers"
	case FuncReadWriteRegisters:
		return "Read/Write Multiple Registers"
	case FuncDiagnostics:
		return "Diagnostics"
	case FuncEncapsulated:
		return "Read Device Identification"
	}
	return fmt.Sprintf("Function %02X", fc)
}

// IsBitFunction reports whether fc reads single-bit data.
func IsBitFunction(fc byte) bool {
	return fc == FuncReadCoils || fc == FuncReadDiscreteInputs
}

func checkRange(address, quantity, max int) error {
	if quantity < 1 || quantity > max {
		return fmt.Errorf("quantity %d out of range 1..%d", quantity, max)
	}
	if address < 0 || address+quantity > 0x10000 {
		return fmt.Errorf("address range %d..%d exceeds 0..65535", address, address+quantity-1)
	}
	return nil
}

// ReadRequest builds a request for function codes 01..04.
func ReadRequest(fc byte, address, quantity int) (PDU, error) {
	max := MaxReadRegisters
	switch fc {
	case FuncReadCoils, FuncReadDiscreteInputs:
		max = MaxReadBits
	case FuncReadHoldingRegisters, FuncReadInputRegisters:
	default:
		return PDU{}, fmt.Errorf("function %02X is not a read function", fc)
	}
	if err := checkRange(address, quantity, max); err != nil {
		return PDU{}, err
	}
	d := make([]byte, 4)
	binary.BigEndian.PutUint16(d[0:], uint16(address))
	binary.BigEndian.PutUint16(d[2:], uint16(quantity))
	return PDU{Function: fc, Data: d}, nil
}

// WriteSingleCoilRequest builds an FC05 request.
func WriteSingleCoilRequest(address int, on bool) (PDU, error) {
	if err := checkRange(address, 1, 1); err != nil {
		return PDU{}, err
	}
	d := make([]byte, 4)
	binary.BigEndian.PutUint16(d[0:], uint16(address))
	if on {
		d[2] = 0xFF
	}
	return PDU{Function: FuncWriteSingleCoil, Data: d}, nil
}

// WriteSingleRegisterRequest builds an FC06 request.
func WriteSingleRegisterRequest(address int, value uint16) (PDU, error) {
	if err := checkRange(address, 1, 1); err != nil {
		return PDU{}, err
	}
	d := make([]byte, 4)
	binary.BigEndian.PutUint16(d[0:], uint16(address))
	binary.BigEndian.PutUint16(d[2:], value)
	return PDU{Function: FuncWriteSingleRegister, Data: d}, nil
}

// WriteMultipleCoilsRequest builds an FC15 request.
func WriteMultipleCoilsRequest(address int, values []bool) (PDU, error) {
	if err := checkRange(address, len(values), MaxWriteBits); err != nil {
		return PDU{}, err
	}
	packed := PackBits(values)
	d := make([]byte, 5+len(packed))
	binary.BigEndian.PutUint16(d[0:], uint16(address))
	binary.BigEndian.PutUint16(d[2:], uint16(len(values)))
	d[4] = byte(len(packed))
	copy(d[5:], packed)
	return PDU{Function: FuncWriteMultipleCoils, Data: d}, nil
}

// WriteMultipleRegistersRequest builds an FC16 request.
func WriteMultipleRegistersRequest(address int, values []uint16) (PDU, error) {
	if err := checkRange(address, len(values), MaxWriteRegisters); err != nil {
		return PDU{}, err
	}
	d := make([]byte, 5+2*len(values))
	binary.BigEndian.PutUint16(d[0:], uint16(address))
	binary.BigEndian.PutUint16(d[2:], uint16(len(values)))
	d[4] = byte(2 * len(values))
	for i, v := range values {
		binary.BigEndian.PutUint16(d[5+2*i:], v)
	}
	return PDU{Function: FuncWriteMultipleRegisters, Data: d}, nil
}

// ReadWriteRegistersRequest builds a FC23 request: the slave writes values
// at writeAddress, then returns readQty registers from readAddress.
func ReadWriteRegistersRequest(readAddress, readQty, writeAddress int, values []uint16) (PDU, error) {
	if err := checkRange(readAddress, readQty, MaxReadRegisters); err != nil {
		return PDU{}, fmt.Errorf("read: %w", err)
	}
	if err := checkRange(writeAddress, len(values), MaxRWWrite); err != nil {
		return PDU{}, fmt.Errorf("write: %w", err)
	}
	d := make([]byte, 9+2*len(values))
	binary.BigEndian.PutUint16(d[0:], uint16(readAddress))
	binary.BigEndian.PutUint16(d[2:], uint16(readQty))
	binary.BigEndian.PutUint16(d[4:], uint16(writeAddress))
	binary.BigEndian.PutUint16(d[6:], uint16(len(values)))
	d[8] = byte(2 * len(values))
	for i, v := range values {
		binary.BigEndian.PutUint16(d[9+2*i:], v)
	}
	return PDU{Function: FuncReadWriteRegisters, Data: d}, nil
}

// CheckResponse verifies that resp answers req, decoding exception responses.
func CheckResponse(req, resp PDU) error {
	if resp.Function == req.Function|0x80 {
		if len(resp.Data) < 1 {
			return invalid("truncated exception response")
		}
		return &ExceptionError{Function: req.Function, Code: resp.Data[0]}
	}
	if resp.Function != req.Function {
		return invalid("function code %02X does not match request %02X", resp.Function, req.Function)
	}
	switch req.Function {
	case FuncReadCoils, FuncReadDiscreteInputs, FuncReadHoldingRegisters, FuncReadInputRegisters:
		if len(resp.Data) < 1 || int(resp.Data[0]) != len(resp.Data)-1 {
			return invalid("byte count mismatch")
		}
		qty := int(binary.BigEndian.Uint16(req.Data[2:]))
		want := 2 * qty
		if IsBitFunction(req.Function) {
			want = (qty + 7) / 8
		}
		if int(resp.Data[0]) != want {
			return invalid("expected %d data bytes, got %d", want, resp.Data[0])
		}
	case FuncReadWriteRegisters:
		qty := int(binary.BigEndian.Uint16(req.Data[2:]))
		if len(resp.Data) < 1 || int(resp.Data[0]) != len(resp.Data)-1 || int(resp.Data[0]) != 2*qty {
			return invalid("expected %d data bytes", 2*qty)
		}
	case FuncWriteSingleCoil, FuncWriteSingleRegister:
		if len(resp.Data) != 4 || string(resp.Data) != string(req.Data[:4]) {
			return invalid("write response does not echo the request")
		}
	case FuncWriteMultipleCoils, FuncWriteMultipleRegisters:
		if len(resp.Data) != 4 || string(resp.Data) != string(req.Data[:4]) {
			return invalid("write response does not echo address/quantity")
		}
	case FuncDiagnostics:
		if len(resp.Data) != 4 || string(resp.Data[:2]) != string(req.Data[:2]) {
			return invalid("diagnostics response does not echo the sub-function")
		}
		if binary.BigEndian.Uint16(req.Data) == DiagReturnQueryData && string(resp.Data) != string(req.Data) {
			return invalid("echo data differs")
		}
	case FuncEncapsulated:
		if len(resp.Data) < 6 || len(req.Data) < 1 || resp.Data[0] != req.Data[0] {
			return invalid("MEI type does not match the request")
		}
	}
	return nil
}

// DecodeBits extracts quantity bits from a validated FC01/02 response.
func DecodeBits(resp PDU, quantity int) []bool {
	return UnpackBits(resp.Data[1:], quantity)
}

// DecodeRegisters extracts registers from a validated FC03/04 response.
func DecodeRegisters(resp PDU) []uint16 {
	n := int(resp.Data[0]) / 2
	out := make([]uint16, n)
	for i := range out {
		out[i] = binary.BigEndian.Uint16(resp.Data[1+2*i:])
	}
	return out
}

// PackBits packs bools LSB-first as specified by Modbus.
func PackBits(values []bool) []byte {
	out := make([]byte, (len(values)+7)/8)
	for i, v := range values {
		if v {
			out[i/8] |= 1 << (i % 8)
		}
	}
	return out
}

// UnpackBits unpacks n LSB-first bits.
func UnpackBits(data []byte, n int) []bool {
	out := make([]bool, n)
	for i := 0; i < n && i/8 < len(data); i++ {
		out[i] = data[i/8]&(1<<(i%8)) != 0
	}
	return out
}
