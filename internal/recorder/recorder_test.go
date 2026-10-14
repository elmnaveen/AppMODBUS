package recorder

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func meta(addr int, name string) Meta {
	id := SeriesID("TCP 10.0.0.5:502", 3, 4, addr, "float_abcd")
	return Meta{ID: id, Connection: "TCP 10.0.0.5:502", SlaveID: 3, Function: 4, Address: addr, Format: "float_abcd", Name: name, Unit: "V"}
}

var day0 = time.Date(2026, 10, 10, 23, 59, 58, 0, time.UTC)

func TestRecordReadAcrossDays(t *testing.T) {
	r, _ := Open(t.TempDir())
	defer r.Close()
	m := meta(0, "Voltage")
	for i := 0; i < 5; i++ { // crosses midnight UTC
		if err := r.Record(m, day0.Add(time.Duration(i)*time.Second), 230+float64(i)); err != nil {
			t.Fatal(err)
		}
	}
	all, err := r.Read(m.ID, 0, 1<<62)
	if err != nil || len(all) != 5 || all[4].V != 234 {
		t.Fatalf("%v %v", all, err)
	}
	part, _ := r.Read(m.ID, day0.Add(time.Second).UnixMilli(), day0.Add(3*time.Second).UnixMilli())
	if len(part) != 3 || part[0].V != 231 || part[2].V != 233 {
		t.Fatalf("range %v", part)
	}
	if days := dayFiles(filepath.Join(r.Root(), m.ID)); len(days) != 2 {
		t.Fatalf("days %v", days)
	}
	s := r.Series()
	if len(s) != 1 || s[0].Samples != 5 || s[0].Bytes != 80 || s[0].First != day0.UnixMilli() || s[0].Name != "Voltage" {
		t.Fatalf("series %+v", s)
	}
	// The clock stepping back must not break time order.
	_ = r.Record(m, day0, 1)
	all, _ = r.Read(m.ID, 0, 1<<62)
	if all[5].T <= all[4].T {
		t.Fatal("timestamps not increasing")
	}
}

func TestPartialRecordAfterCrash(t *testing.T) {
	root := t.TempDir()
	r, _ := Open(root)
	m := meta(1, "x")
	r.Record(m, day0, 1)
	r.Record(m, day0.Add(time.Second), 2)
	r.Close()
	path := filepath.Join(root, m.ID, dayOf(day0.UnixMilli())+dataExt)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	f.Write([]byte{1, 2, 3, 4, 5}) // torn write
	f.Close()

	r2, _ := Open(root)
	defer r2.Close()
	if got, _ := r2.Read(m.ID, 0, 1<<62); len(got) != 2 {
		t.Fatalf("read %v", got)
	}
	r2.Record(m, day0.Add(2*time.Second), 3)
	got, _ := r2.Read(m.ID, 0, 1<<62)
	if len(got) != 3 || got[2].V != 3 {
		t.Fatalf("after append %v", got)
	}
}

func TestQueryKeepsSpikes(t *testing.T) {
	r, _ := Open(t.TempDir())
	defer r.Close()
	m := meta(2, "y")
	for i := 0; i < 1000; i++ {
		v := 10.0
		if i == 500 {
			v = 999
		}
		r.Record(m, day0.Add(time.Duration(i)*time.Millisecond*100), v)
	}
	q, _ := r.Query(m.ID, day0.UnixMilli(), day0.Add(100*time.Second).UnixMilli(), 100)
	if len(q) > 100 || len(q) < 40 {
		t.Fatalf("points %d", len(q))
	}
	spike := false
	for i, s := range q {
		spike = spike || s.V == 999
		if i > 0 && s.T < q[i-1].T {
			t.Fatal("unordered")
		}
	}
	if !spike {
		t.Fatal("downsampling lost the spike")
	}
}

func TestEnforceRetentionAndSize(t *testing.T) {
	r, _ := Open(t.TempDir())
	defer r.Close()
	a, b := meta(0, "a"), meta(2, "b")
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	for d := 9; d >= 0; d-- { // oldest first, as real recordings are
		ts := now.AddDate(0, 0, -d)
		for i := 0; i < 100; i++ {
			r.Record(a, ts.Add(time.Duration(i)*time.Second), 1)
			r.Record(b, ts.Add(time.Duration(i)*time.Second), 2)
		}
	}
	r.Flush()
	if u := r.Usage(); u != 10*2*100*16 {
		t.Fatalf("usage %d", u)
	}
	// Keep 5 days: today and the 4 days before... cutoff is now-5d.
	freed, _ := r.Enforce(5, 0, now)
	if freed != 4*2*100*16 {
		t.Fatalf("retention freed %d", freed)
	}
	// Cap at 2 days worth: oldest go first, today always stays.
	r.Enforce(0, 2*2*100*16, now)
	for _, s := range r.Series() {
		if s.Bytes > 2*100*16 || s.Last < now.UnixMilli() {
			t.Fatalf("after cap %+v", s)
		}
	}
	// A huge cap below today's size keeps today.
	r.Enforce(0, 1, now)
	if len(r.Series()) != 2 {
		t.Fatal("today's data must survive the size cap")
	}
}

func TestDeleteRangeAndSeries(t *testing.T) {
	r, _ := Open(t.TempDir())
	defer r.Close()
	m, n := meta(0, "a"), meta(4, "b")
	for i := 0; i < 10; i++ {
		r.Record(m, day0.Add(time.Duration(i)*time.Second), float64(i))
		r.Record(n, day0.Add(time.Duration(i)*time.Second), float64(i))
	}
	if err := r.DeleteRange([]string{m.ID}, day0.Add(2*time.Second).UnixMilli(), day0.Add(5*time.Second).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	got, _ := r.Read(m.ID, 0, 1<<62)
	if len(got) != 6 || got[2].V != 6 {
		t.Fatalf("after range delete %v", got)
	}
	if other, _ := r.Read(n.ID, 0, 1<<62); len(other) != 10 {
		t.Fatal("other series touched")
	}
	// Writing continues after a delete.
	r.Record(m, day0.Add(20*time.Second), 42)
	if got, _ := r.Read(m.ID, 0, 1<<62); got[len(got)-1].V != 42 {
		t.Fatal("append after delete")
	}
	r.DeleteSeries([]string{m.ID})
	if s := r.Series(); len(s) != 1 || s[0].ID != n.ID {
		t.Fatalf("series %+v", s)
	}
	if err := r.DeleteSeries([]string{"../x"}); err == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestExports(t *testing.T) {
	dir := t.TempDir()
	r, _ := Open(filepath.Join(dir, "rec"))
	defer r.Close()
	v, a := meta(0, "Voltage"), meta(2, "Current")
	a.Unit = "A"
	for i := 0; i < 3; i++ {
		ts := day0.Add(time.Duration(i) * time.Second)
		r.Record(v, ts, 230.5+float64(i))
		r.Record(a, ts, 5)
	}
	csvPath := filepath.Join(dir, "out.csv")
	n, err := r.ExportCSV(csvPath, []string{v.ID, a.ID}, 0, 1<<62, time.UTC)
	if err != nil || n != 3 {
		t.Fatalf("csv %d %v", n, err)
	}
	b, _ := os.ReadFile(csvPath)
	lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(string(b), "\xef\xbb\xbf")), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[0], `Time,"Voltage [V] (TCP 10.0.0.5:502 · slave 3, FC04 0)"`) || !strings.Contains(lines[0], "Current [A]") ||
		lines[1] != "2026-10-10 23:59:58.000,230.5,5" {
		t.Fatalf("csv:\n%s", b)
	}
	xPath := filepath.Join(dir, "out.xlsx")
	if n, err := r.ExportXLSX(xPath, []string{v.ID, a.ID}, 0, 1<<62, time.UTC); err != nil || n != 3 {
		t.Fatalf("xlsx %d %v", n, err)
	}
	zr, err := zip.OpenReader(xPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var sheet string
	for _, f := range zr.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			rc, _ := f.Open()
			bb, _ := io.ReadAll(rc)
			rc.Close()
			sheet = string(bb)
		}
	}
	if !strings.Contains(sheet, `<c r="B2"><v>230.5</v></c>`) || !strings.Contains(sheet, "Voltage [V]") || !strings.Contains(sheet, `<c r="A4" s="1">`) {
		t.Fatalf("sheet: %s", sheet)
	}
	if _, err := r.ExportCSV(csvPath, nil, 0, 1, time.UTC); err == nil {
		t.Fatal("empty selection must fail")
	}
	for in, want := range map[float64]string{229.70000000000002: "229.7", 50: "50", 0.000123: "0.000123", 1e15: "1000000000000000"} {
		if got := formatValue(in, 12); got != want {
			t.Errorf("formatValue(%v) = %s", in, got)
		}
	}
	if got := formatValue(float64(float32(234.207)), 7); got != "234.207" {
		t.Errorf("float32 value %s", got)
	}
	if colName(0) != "A" || colName(25) != "Z" || colName(26) != "AA" || colName(701) != "ZZ" {
		t.Fatal("colName")
	}
}
