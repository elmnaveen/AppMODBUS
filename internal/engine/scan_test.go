package engine

import (
	"net"
	"reflect"
	"strconv"
	"testing"
	"time"

	"appmodbus/internal/modbus"
	"appmodbus/internal/simulator"
)

func startSimCfg(t *testing.T, e *Engine, cfg simulator.Config) int {
	t.Helper()
	cfg.Host, cfg.Port = "127.0.0.1", 0
	if cfg.SlaveID == 0 {
		cfg.SlaveID = 1
	}
	if err := e.Sim.Start(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Sim.Stop)
	_, port, _ := net.SplitHostPort(e.Sim.Addr())
	p, _ := strconv.Atoi(port)
	return p
}

func waitScan(t *testing.T, e *Engine) ScanState {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if st := e.ScanState(); !st.Running {
			if st.Error != "" {
				t.Fatalf("scan failed: %s", st.Error)
			}
			return st
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("scan did not finish")
	return ScanState{}
}

func TestIDScan(t *testing.T) {
	for _, mode := range []modbus.Mode{modbus.ModeTCP, modbus.ModeRTUOverTCP} {
		t.Run(string(mode), func(t *testing.T) {
			e := New(nil)
			defer e.Shutdown()
			port := startSimCfg(t, e, simulator.Config{Mode: mode, SlaveID: 3})
			conn, _ := e.SaveConnection(ConnectionConfig{Mode: mode, Host: "127.0.0.1", Port: port, TimeoutMs: 500})
			if err := e.StartIDScan(IDScanRequest{ConnectionID: conn.ID, From: 1, To: 5, Function: 3, TimeoutMs: 60}); err != nil {
				t.Fatal(err)
			}
			st := waitScan(t, e)
			if st.Done != 5 || len(st.IDs) != 1 || st.IDs[0].SlaveID != 3 || st.IDs[0].Status != "ok" {
				t.Fatalf("scan result %+v", st)
			}
		})
	}
}

func TestMapScan(t *testing.T) {
	e := New(nil)
	defer e.Shutdown()
	port := startSimCfg(t, e, simulator.Config{Mode: modbus.ModeTCP, Limits: map[simulator.Table]string{
		simulator.HoldingRegisters: "5-37, 1000-1019",
		simulator.Coils:            "0-15",
		simulator.DiscreteInputs:   "0-7, 40-42",
	}})
	conn, _ := e.SaveConnection(ConnectionConfig{Mode: modbus.ModeTCP, Host: "127.0.0.1", Port: port, TimeoutMs: 500})
	if err := e.StartMapScan(MapScanRequest{ConnectionID: conn.ID, SlaveID: 1, Functions: []int{1, 2, 3}, From: 0, To: 1999}); err != nil {
		t.Fatal(err)
	}
	st := waitScan(t, e)
	want := []MapRange{{1, 0, 15}, {2, 0, 7}, {2, 40, 42}, {3, 5, 37}, {3, 1000, 1019}}
	if !reflect.DeepEqual(st.Ranges, want) {
		t.Fatalf("ranges %+v, want %+v", st.Ranges, want)
	}
	if st.Done != st.Total {
		t.Fatalf("progress %d/%d", st.Done, st.Total)
	}
}

func TestNetworkScanAndDeviceTools(t *testing.T) {
	e := New(nil)
	defer e.Shutdown()
	port := startSimCfg(t, e, simulator.Config{Mode: modbus.ModeTCP})
	if err := e.StartNetworkScan(NetworkScanRequest{Range: "127.0.0.1", Port: port, Units: "1,2", TimeoutMs: 300, Identify: true}); err != nil {
		t.Fatal(err)
	}
	st := waitScan(t, e)
	if len(st.Hosts) != 1 || !reflect.DeepEqual(st.Hosts[0].Units, []int{1}) || st.Hosts[0].Status != "modbus" {
		t.Fatalf("hosts %+v", st.Hosts)
	}
	if !reflect.DeepEqual(st.Hosts[0].Identity, []string{"Modbus Studio", "SIM-1", "1.0"}) {
		t.Fatalf("identity %+v", st.Hosts[0].Identity)
	}

	// Device identification over RTU framing needs the variable-length reader.
	e.Sim.Stop()
	port = startSimCfg(t, e, simulator.Config{Mode: modbus.ModeRTUOverTCP})
	conn, _ := e.SaveConnection(ConnectionConfig{Mode: modbus.ModeRTUOverTCP, Host: "127.0.0.1", Port: port, TimeoutMs: 500})
	cl, timeout, _ := e.Client(conn.ID)
	id, err := cl.ReadDeviceIdentification(1, modbus.DeviceIDRegular, timeout)
	if err != nil || len(id.Objects) != 7 || id.Objects[4].Value != "Modbus Studio slave simulator" {
		t.Fatalf("device id %+v, %v", id, err)
	}
	if v, err := cl.Diagnostic(1, modbus.DiagReturnQueryData, 0x1234, timeout); err != nil || v != 0x1234 {
		t.Fatalf("echo %x %v", v, err)
	}
	if v, err := cl.Diagnostic(1, modbus.DiagServerMessageCount, 0, timeout); err != nil || v != 3 {
		t.Fatalf("message count %d %v (want 3)", v, err)
	}

	stats := e.Statistics(conn.ID)
	var cs *ConnectionStats
	for i := range stats {
		if stats[i].ID == conn.ID {
			cs = &stats[i]
		}
	}
	if cs == nil || cs.Stats.Requests != 3 || cs.Stats.OK != 3 || len(cs.Stats.Recent) != 3 || cs.Stats.Histogram[0].Count+cs.Stats.Histogram[1].Count+cs.Stats.Histogram[2].Count == 0 {
		t.Fatalf("stats %+v", cs)
	}
	e.ResetStatistics(conn.ID)
	if s := e.Statistics("")[0].Stats; s.Requests != 0 {
		t.Fatalf("after reset %+v", s)
	}
}

func TestParseIPRange(t *testing.T) {
	cases := map[string]int{"192.168.1.1-254": 254, "10.0.0.0/24": 254, "10.0.0.250-10.0.1.5": 12, "1.2.3.4": 1, "10.0.0.0/31": 2}
	for in, n := range cases {
		ips, err := ParseIPRange(in)
		if err != nil || len(ips) != n {
			t.Errorf("%s: %d %v, want %d", in, len(ips), err, n)
		}
	}
	for _, bad := range []string{"10.0.0.0/16", "1.2.3.9-4", "x", "1.2.3.4-1.2.3.1"} {
		if _, err := ParseIPRange(bad); err == nil {
			t.Errorf("%s: expected an error", bad)
		}
	}
}

func TestReadWrite23(t *testing.T) {
	for _, mode := range []modbus.Mode{modbus.ModeTCP, modbus.ModeRTUOverTCP, modbus.ModeASCIITCP, modbus.ModeUDP} {
		t.Run(string(mode), func(t *testing.T) {
			e := New(nil)
			defer e.Shutdown()
			port := startSimCfg(t, e, simulator.Config{Mode: mode})
			e.Sim.Store.Write(simulator.HoldingRegisters, 0, []int{1, 2, 3, 4})
			conn, _ := e.SaveConnection(ConnectionConfig{Mode: mode, Host: "127.0.0.1", Port: port, TimeoutMs: 500})
			got, err := e.ReadWrite(ReadWriteRequest{ConnectionID: conn.ID, SlaveID: 1, ReadAddress: 0, ReadQuantity: 4, WriteAddress: 2, Values: []int{30, 40}})
			if err != nil {
				t.Fatal(err)
			}
			// The write happens first, so the read sees it.
			if !reflect.DeepEqual(got, []int{1, 2, 30, 40}) {
				t.Fatalf("read back %v", got)
			}
		})
	}
}
