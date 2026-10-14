package modbus

import (
	"encoding/binary"
	"fmt"
	"time"
)

// Function codes for diagnostics and device identification.
const (
	FuncDiagnostics  byte = 0x08
	FuncEncapsulated byte = 0x2B // MEI transport
	MEIReadDeviceID  byte = 0x0E
)

// Diagnostics (FC08) sub-functions.
const (
	DiagReturnQueryData       uint16 = 0x00
	DiagRestartComm           uint16 = 0x01
	DiagClearCounters         uint16 = 0x0A
	DiagBusMessageCount       uint16 = 0x0B
	DiagBusCommErrorCount     uint16 = 0x0C
	DiagBusExceptionCount     uint16 = 0x0D
	DiagServerMessageCount    uint16 = 0x0E
	DiagServerNoResponseCount uint16 = 0x0F
	DiagServerNAKCount        uint16 = 0x10
	DiagServerBusyCount       uint16 = 0x11
	DiagBusCharOverrunCount   uint16 = 0x12
)

// DiagCounter names a FC08 counter sub-function.
type DiagCounter struct {
	Sub  uint16 `json:"sub"`
	Name string `json:"name"`
}

// DiagCounters lists the counters a serial-line slave keeps (spec 6.8).
var DiagCounters = []DiagCounter{
	{DiagBusMessageCount, "Bus messages"},
	{DiagBusCommErrorCount, "Bus communication errors (CRC)"},
	{DiagBusExceptionCount, "Exception responses"},
	{DiagServerMessageCount, "Messages addressed to the slave"},
	{DiagServerNoResponseCount, "No response (broadcasts)"},
	{DiagServerNAKCount, "NAK responses"},
	{DiagServerBusyCount, "Busy responses"},
	{DiagBusCharOverrunCount, "Character overruns"},
}

// DiagnosticsRequest builds a FC08 request with one data word.
func DiagnosticsRequest(sub, data uint16) PDU {
	d := make([]byte, 4)
	binary.BigEndian.PutUint16(d, sub)
	binary.BigEndian.PutUint16(d[2:], data)
	return PDU{Function: FuncDiagnostics, Data: d}
}

// Diagnostic runs a FC08 sub-function and returns the data word of the
// response (the echo, or the counter value).
func (c *Client) Diagnostic(unit byte, sub, data uint16, timeout time.Duration) (uint16, error) {
	resp, err := c.Do(unit, DiagnosticsRequest(sub, data), timeout)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(resp.Data[2:]), nil
}

// Read device identification codes (FC43/14).
const (
	DeviceIDBasic    byte = 0x01
	DeviceIDRegular  byte = 0x02
	DeviceIDExtended byte = 0x03
	DeviceIDSpecific byte = 0x04
)

// DeviceObject is one identification object.
type DeviceObject struct {
	ID    byte   `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// DeviceID is the result of reading a device's identification.
type DeviceID struct {
	Conformity byte           `json:"conformity"`
	Objects    []DeviceObject `json:"objects"`
}

// DeviceObjectName names the standard objects 0x00..0x06.
func DeviceObjectName(id byte) string {
	switch id {
	case 0x00:
		return "Vendor name"
	case 0x01:
		return "Product code"
	case 0x02:
		return "Revision"
	case 0x03:
		return "Vendor URL"
	case 0x04:
		return "Product name"
	case 0x05:
		return "Model name"
	case 0x06:
		return "User application name"
	}
	if id >= 0x80 {
		return fmt.Sprintf("Private object %02X", id)
	}
	return fmt.Sprintf("Reserved object %02X", id)
}

// ReadDeviceIDRequest builds a FC43/14 request.
func ReadDeviceIDRequest(code, objectID byte) PDU {
	return PDU{Function: FuncEncapsulated, Data: []byte{MEIReadDeviceID, code, objectID}}
}

// deviceIDPage is one response of a (possibly multi-part) FC43/14 read.
type deviceIDPage struct {
	conformity byte
	more       bool
	next       byte
	objects    []DeviceObject
}

func parseDeviceID(resp PDU) (deviceIDPage, error) {
	d := resp.Data
	if len(d) < 6 || d[0] != MEIReadDeviceID {
		return deviceIDPage{}, invalid("short device identification response")
	}
	pg := deviceIDPage{conformity: d[2], more: d[3] == 0xFF, next: d[4]}
	n := int(d[5])
	i := 6
	for k := 0; k < n; k++ {
		if i+2 > len(d) || i+2+int(d[i+1]) > len(d) {
			return deviceIDPage{}, invalid("truncated device identification object")
		}
		id, l := d[i], int(d[i+1])
		pg.objects = append(pg.objects, DeviceObject{ID: id, Name: DeviceObjectName(id), Value: string(d[i+2 : i+2+l])})
		i += 2 + l
	}
	return pg, nil
}

// ReadDeviceIdentification reads all objects of the given category
// (basic, regular or extended), following "more follows" responses.
func (c *Client) ReadDeviceIdentification(unit, code byte, timeout time.Duration) (DeviceID, error) {
	var out DeviceID
	obj := byte(0)
	for page := 0; page < 64; page++ {
		resp, err := c.Do(unit, ReadDeviceIDRequest(code, obj), timeout)
		if err != nil {
			return out, err
		}
		pg, err := parseDeviceID(resp)
		if err != nil {
			return out, err
		}
		out.Conformity = pg.conformity
		out.Objects = append(out.Objects, pg.objects...)
		if !pg.more || pg.next <= obj && page > 0 {
			return out, nil
		}
		obj = pg.next
	}
	return out, nil
}

// EncodeDeviceID builds a FC43/14 response (used by the simulator).
// It returns as many objects from start as fit in one PDU.
func EncodeDeviceID(code, conformity byte, objs []DeviceObject) PDU {
	d := []byte{MEIReadDeviceID, code, conformity, 0, 0, 0}
	for _, o := range objs {
		v := o.Value
		if len(v) > 240 {
			v = v[:240]
		}
		if len(d)+2+len(v) > 252 {
			d[3], d[4] = 0xFF, o.ID
			break
		}
		d = append(d, o.ID, byte(len(v)))
		d = append(d, v...)
		d[5]++
	}
	return PDU{Function: FuncEncapsulated, Data: d}
}
