package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecentKeepsNewestInOrder(t *testing.T) {
	l := Open(t.TempDir(), nil)
	defer l.Close()
	for i := 0; i < MaxEntries+10; i++ {
		l.Infof("app", "entry %d", i)
	}
	all := l.Recent(0)
	if len(all) != MaxEntries {
		t.Fatalf("kept %d entries, want %d", len(all), MaxEntries)
	}
	if all[0].Message != "entry 10" || all[len(all)-1].Message != "entry 5009" {
		t.Fatalf("oldest %q newest %q", all[0].Message, all[len(all)-1].Message)
	}
	last := l.Recent(3)
	if len(last) != 3 || last[2].Message != "entry 5009" || last[0].ID+2 != last[2].ID {
		t.Fatalf("recent(3) = %+v", last)
	}
	l.Clear()
	if n := len(l.Recent(0)); n != 0 {
		t.Fatalf("after clear: %d entries", n)
	}
}

func TestLevelsAndEmit(t *testing.T) {
	var got []Entry
	l := Open(t.TempDir(), func(e Entry) { got = append(got, e) })
	defer l.Close()
	l.Add("bogus", "ui", "  hello  ")
	l.Warnf("poll", "timeout on %s", "Meter")
	l.Errorf("connection", "refused")
	if len(got) != 3 || got[0].Level != Info || got[0].Message != "hello" || got[1].Level != Warn || got[1].Message != "timeout on Meter" || got[2].Level != Error {
		t.Fatalf("emitted %+v", got)
	}
}

func TestDailyFilesAndPrune(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, filePrefix+"2000-01-01"+fileSuffix)
	keep := filepath.Join(dir, filePrefix+time.Now().AddDate(0, 0, -2).Format("2006-01-02")+fileSuffix)
	other := filepath.Join(dir, "notes.txt")
	for _, p := range []string{old, keep, other} {
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	day := time.Date(2026, 10, 14, 23, 59, 59, 0, time.Local)
	l := Open(dir, nil)
	l.now = func() time.Time { return day }
	l.Infof("app", "before midnight")
	day = day.Add(2 * time.Second)
	l.Warnf("poll", "after midnight")
	l.Close()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("old log file was not pruned")
	}
	for _, p := range []string{keep, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was removed", filepath.Base(p))
		}
	}
	a, _ := os.ReadFile(filepath.Join(dir, filePrefix+"2026-10-14"+fileSuffix))
	b, _ := os.ReadFile(filepath.Join(dir, filePrefix+"2026-10-15"+fileSuffix))
	if !strings.Contains(string(a), "INFO  [app] before midnight") || !strings.Contains(string(b), "WARN  [poll] after midnight") {
		t.Fatalf("files:\n%s\n%s", a, b)
	}
}

func TestFormat(t *testing.T) {
	ts := time.Date(2026, 10, 14, 9, 5, 3, 7_000_000, time.Local).UnixMilli()
	got := Format([]Entry{{Time: ts, Level: Error, Source: "write", Message: "exception 02"}})
	if got != "2026-10-14 09:05:03.007 ERROR [write] exception 02\n" {
		t.Fatalf("got %q", got)
	}
}
