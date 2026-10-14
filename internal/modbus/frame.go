package modbus

import "encoding/binary"

// CRC16 computes the Modbus RTU CRC (poly 0xA001, init 0xFFFF).
func CRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = crc>>1 ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// EncodeRTU builds an RTU ADU: unit, PDU, CRC (low byte first).
func EncodeRTU(unit byte, p PDU) []byte {
	b := make([]byte, 0, 4+len(p.Data))
	b = append(b, unit, p.Function)
	b = append(b, p.Data...)
	crc := CRC16(b)
	return append(b, byte(crc), byte(crc>>8))
}

// DecodeRTU validates the CRC of an RTU ADU and splits it.
func DecodeRTU(adu []byte) (unit byte, p PDU, err error) {
	if len(adu) < 4 {
		return 0, PDU{}, invalid("RTU frame too short (%d bytes)", len(adu))
	}
	n := len(adu) - 2
	want := CRC16(adu[:n])
	got := uint16(adu[n]) | uint16(adu[n+1])<<8
	if want != got {
		return 0, PDU{}, invalid("CRC error (got %04X, want %04X)", got, want)
	}
	data := make([]byte, n-2)
	copy(data, adu[2:n])
	return adu[0], PDU{Function: adu[1], Data: data}, nil
}

// RTUResponseLength returns the total length of an RTU response frame given
// its first 3 bytes, or 0 if the length cannot be derived.
func RTUResponseLength(head []byte) int {
	if len(head) < 3 {
		return 0
	}
	fc := head[1]
	if fc&0x80 != 0 {
		return 5
	}
	switch fc {
	case FuncReadCoils, FuncReadDiscreteInputs, FuncReadHoldingRegisters, FuncReadInputRegisters:
		return 5 + int(head[2])
	case FuncWriteSingleCoil, FuncWriteSingleRegister, FuncWriteMultipleCoils, FuncWriteMultipleRegisters, FuncDiagnostics:
		return 8
	}
	return 0
}

// DeviceIDFrameLength works out the length of an RTU FC43/14 response
// from the bytes received so far. It returns the total length, or 0 and
// how many bytes are needed before it can tell.
func DeviceIDFrameLength(b []byte) (total, need int) {
	const head = 8 // unit, fc, MEI, code, conformity, more, next, count
	if len(b) < head {
		return 0, head
	}
	i := head
	for k := 0; k < int(b[7]); k++ {
		if len(b) < i+2 {
			return 0, i + 2
		}
		i += 2 + int(b[i+1])
	}
	return i + 2, i + 2
}

// RTURequestLength returns the total length of an RTU request frame given
// its first 7 bytes (or fewer when that is enough), or 0 if unknown.
// need reports how many header bytes must be present to decide.
func RTURequestLength(head []byte) (length int, need int) {
	if len(head) < 2 {
		return 0, 2
	}
	switch head[1] {
	case FuncReadCoils, FuncReadDiscreteInputs, FuncReadHoldingRegisters, FuncReadInputRegisters,
		FuncWriteSingleCoil, FuncWriteSingleRegister:
		return 8, 2
	case FuncWriteMultipleCoils, FuncWriteMultipleRegisters:
		if len(head) < 7 {
			return 0, 7
		}
		return 9 + int(head[6]), 7
	case FuncDiagnostics:
		return 8, 2
	case FuncEncapsulated:
		return 7, 2
	}
	return 0, 2
}

// MBAPHeaderLen is the size of the Modbus TCP header.
const MBAPHeaderLen = 7

// EncodeMBAP builds a Modbus TCP ADU.
func EncodeMBAP(txID uint16, unit byte, p PDU) []byte {
	b := make([]byte, MBAPHeaderLen+1+len(p.Data))
	binary.BigEndian.PutUint16(b[0:], txID)
	binary.BigEndian.PutUint16(b[2:], 0)
	binary.BigEndian.PutUint16(b[4:], uint16(2+len(p.Data)))
	b[6] = unit
	b[7] = p.Function
	copy(b[8:], p.Data)
	return b
}

// MBAPHeader is a decoded Modbus TCP header.
type MBAPHeader struct {
	TxID     uint16
	Protocol uint16
	Length   uint16 // unit id + PDU
	Unit     byte
}

// DecodeMBAPHeader decodes the 7-byte header.
func DecodeMBAPHeader(b []byte) (MBAPHeader, error) {
	if len(b) < MBAPHeaderLen {
		return MBAPHeader{}, invalid("MBAP header too short")
	}
	h := MBAPHeader{
		TxID:     binary.BigEndian.Uint16(b[0:]),
		Protocol: binary.BigEndian.Uint16(b[2:]),
		Length:   binary.BigEndian.Uint16(b[4:]),
		Unit:     b[6],
	}
	if h.Protocol != 0 {
		return h, invalid("protocol id %d is not Modbus", h.Protocol)
	}
	if h.Length < 2 || h.Length > 254 {
		return h, invalid("MBAP length %d out of range", h.Length)
	}
	return h, nil
}

// HexString formats bytes as space separated upper case hex.
func HexString(b []byte) string {
	const digits = "0123456789ABCDEF"
	if len(b) == 0 {
		return ""
	}
	out := make([]byte, 0, len(b)*3-1)
	for i, c := range b {
		if i > 0 {
			out = append(out, ' ')
		}
		out = append(out, digits[c>>4], digits[c&0x0F])
	}
	return string(out)
}

// SerialFrameTimes returns the RTU inter-frame silence (t3.5) for a serial
// line, following the spec's fixed 1.75 ms above 19200 baud.
func SerialFrameTimes(baud, dataBits int, parity string, stopBits int) (t35 float64) {
	if baud <= 0 {
		return 0
	}
	if baud > 19200 {
		return 1.75
	}
	bits := 1 + dataBits + stopBits
	if parity != "N" && parity != "" {
		bits++
	}
	return 3.5 * float64(bits) * 1000 / float64(baud)
}
