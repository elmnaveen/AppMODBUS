package main

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"appmodbus/internal/engine"
	"appmodbus/internal/modbus"
)

// ---- statistics ----

// Statistics returns per-connection and per-window statistics. detailID
// names the connection whose histogram and recent samples are included.
func (a *App) Statistics(detailID string) []engine.ConnectionStats {
	return a.engine.Statistics(detailID)
}

// ResetStatistics clears the counters of a connection ("" = all).
func (a *App) ResetStatistics(connID string) { a.engine.ResetStatistics(connID) }

// ---- scans ----

func (a *App) StartIDScan(r engine.IDScanRequest) error   { return a.engine.StartIDScan(r) }
func (a *App) StartMapScan(r engine.MapScanRequest) error { return a.engine.StartMapScan(r) }
func (a *App) StartNetworkScan(r engine.NetworkScanRequest) error {
	return a.engine.StartNetworkScan(r)
}
func (a *App) StopScan()                   { a.engine.StopScan() }
func (a *App) ScanState() engine.ScanState { return a.engine.ScanState() }

// PollsFromMap opens poll windows for the ranges a map scan found, split
// into blocks a single request can read.
func (a *App) PollsFromMap(connID string, slaveID int, ranges []engine.MapRange) ([]engine.Poll, error) {
	names := map[int]string{1: "Coils", 2: "Inputs", 3: "Holding", 4: "Input regs"}
	var out []engine.Poll
	for _, r := range ranges {
		size := 100
		if r.Function <= 2 {
			size = 800
		}
		for from := r.From; from <= r.To; from += size {
			n := min(size, r.To-from+1)
			p, err := a.engine.CreatePoll(engine.PollConfig{
				Name:         fmt.Sprintf("ID%d · %s %d–%d", slaveID, names[r.Function], from, from+n-1),
				ConnectionID: connID, SlaveID: slaveID, Function: r.Function,
				Address: from, Quantity: n, ScanRateMs: 1000, Format: "signed",
			})
			if err != nil {
				return out, err
			}
			out = append(out, p)
		}
	}
	return out, nil
}

// ---- device tools (FC08, FC43) ----

// EchoResult reports a FC08 "return query data" loop test.
type EchoResult struct {
	Sent   int      `json:"sent"`
	OK     int      `json:"ok"`
	MinMs  float64  `json:"minMs"`
	AvgMs  float64  `json:"avgMs"`
	MaxMs  float64  `json:"maxMs"`
	Errors []string `json:"errors"`
}

// EchoTest sends count FC08/00 requests with random data.
func (a *App) EchoTest(connID string, slaveID, count int) (EchoResult, error) {
	cl, timeout, err := a.engine.Client(connID)
	if err != nil {
		return EchoResult{}, err
	}
	res := EchoResult{Errors: []string{}}
	var sum float64
	for i := 0; i < max(1, min(count, 1000)); i++ {
		v := uint16(rand.Intn(0x10000))
		start := time.Now()
		got, err := cl.Diagnostic(byte(slaveID), modbus.DiagReturnQueryData, v, timeout)
		ms := float64(time.Since(start).Microseconds()) / 1000
		res.Sent++
		if err == nil && got != v {
			err = fmt.Errorf("echo %04X came back as %04X", v, got)
		}
		if err != nil {
			if len(res.Errors) < 20 {
				res.Errors = append(res.Errors, fmt.Sprintf("#%d: %s", i+1, err))
			}
			var ex *modbus.ExceptionError
			if errors.As(err, &ex) && ex.Code == modbus.ExIllegalFunction {
				break // not supported: no point in repeating
			}
			continue
		}
		if res.OK == 0 || ms < res.MinMs {
			res.MinMs = ms
		}
		res.MaxMs = max(res.MaxMs, ms)
		sum += ms
		res.OK++
	}
	if res.OK > 0 {
		res.AvgMs = sum / float64(res.OK)
	}
	return res, nil
}

// CounterValue is one FC08 counter.
type CounterValue struct {
	Sub   uint16 `json:"sub"`
	Name  string `json:"name"`
	Value int    `json:"value"` // -1 when the slave refused the request
	Error string `json:"error"`
}

// DiagCounters reads the serial-line counters of a slave (FC08/0B-12).
func (a *App) DiagCounters(connID string, slaveID int) ([]CounterValue, error) {
	cl, timeout, err := a.engine.Client(connID)
	if err != nil {
		return nil, err
	}
	out := make([]CounterValue, 0, len(modbus.DiagCounters))
	for _, c := range modbus.DiagCounters {
		v, err := cl.Diagnostic(byte(slaveID), c.Sub, 0, timeout)
		cv := CounterValue{Sub: c.Sub, Name: c.Name, Value: int(v)}
		if err != nil {
			cv.Value, cv.Error = -1, err.Error()
			if errors.Is(err, modbus.ErrTimeout) {
				return out, fmt.Errorf("slave %d does not answer: %w", slaveID, err)
			}
		}
		out = append(out, cv)
	}
	return out, nil
}

// ClearDiagCounters resets the slave's counters (FC08/0A).
func (a *App) ClearDiagCounters(connID string, slaveID int) error {
	cl, timeout, err := a.engine.Client(connID)
	if err != nil {
		return err
	}
	_, err = cl.Diagnostic(byte(slaveID), modbus.DiagClearCounters, 0, timeout)
	return err
}

// ReadDeviceIdentification reads FC43/14 objects. code: 1 basic,
// 2 regular, 3 extended.
func (a *App) ReadDeviceIdentification(connID string, slaveID, code int) (modbus.DeviceID, error) {
	cl, timeout, err := a.engine.Client(connID)
	if err != nil {
		return modbus.DeviceID{}, err
	}
	if code < 1 || code > 3 {
		code = 1
	}
	id, err := cl.ReadDeviceIdentification(byte(slaveID), byte(code), timeout)
	if id.Objects == nil {
		id.Objects = []modbus.DeviceObject{}
	}
	return id, err
}
