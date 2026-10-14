// Package recorder stores polled values on disk for charts and export.
//
// Each series (one register of one device) is a directory holding one file
// per UTC day. A file is a sequence of fixed 16-byte records: unix time in
// milliseconds (int64) and the value (float64), both little endian. Records
// are appended in time order, so a time range is found by binary search, a
// day is deleted by removing a file, and a crash can at worst leave a
// partial record at the end, which is ignored on read and trimmed on append.
package recorder

import (
	"bufio"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	recordSize = 16
	dayLayout  = "2006-01-02"
	metaFile   = "series.json"
	dataExt    = ".rec"
)

// Meta describes a recorded series.
type Meta struct {
	ID         string `json:"id"`
	Connection string `json:"connection"` // connection summary, e.g. "TCP 10.0.0.5:502"
	SlaveID    int    `json:"slaveId"`
	Function   int    `json:"function"`
	Address    int    `json:"address"`
	Format     string `json:"format"`
	Name       string `json:"name"`
	Unit       string `json:"unit"`
	Window     string `json:"window"` // poll window name when last recorded
}

// Device identifies the slave a series belongs to.
func (m Meta) Device() string { return fmt.Sprintf("%s · slave %d", m.Connection, m.SlaveID) }

// Label is a readable series name.
func (m Meta) Label() string {
	if m.Name != "" {
		return m.Name
	}
	return fmt.Sprintf("FC%02d %d", m.Function, m.Address)
}

// SeriesID derives a stable ID from where the value comes from and how it
// is decoded, so the same register keeps one history across workspaces.
func SeriesID(connection string, slave, function, address int, format string) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%s|%d|%d|%d|%s", connection, slave, function, address, format)))
	return fmt.Sprintf("s%d-f%d-a%d-%s", slave, function, address, hex.EncodeToString(h[:4]))
}

// Sample is one recorded value.
type Sample struct {
	T int64   `json:"t"` // unix ms
	V float64 `json:"v"`
}

// SeriesInfo is a series with its storage statistics.
type SeriesInfo struct {
	Meta
	Bytes   int64 `json:"bytes"`
	Samples int64 `json:"samples"`
	First   int64 `json:"first"` // unix ms, 0 if empty
	Last    int64 `json:"last"`
}

type openFile struct {
	day string
	f   *os.File
	w   *bufio.Writer
}

// Recorder writes and reads series under a root directory.
type Recorder struct {
	mu    sync.Mutex
	root  string
	files map[string]*openFile
	metas map[string]Meta
	last  map[string]int64 // last written timestamp per series (keeps files ordered)
}

// Open creates a recorder storing data under root.
func Open(root string) (*Recorder, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Recorder{root: root, files: map[string]*openFile{}, metas: map[string]Meta{}, last: map[string]int64{}}, nil
}

// Root returns the storage directory.
func (r *Recorder) Root() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.root
}

// SetRoot switches to a new storage directory. Existing data stays where
// it is.
func (r *Recorder) SetRoot(root string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeAllLocked()
	r.root = root
	r.metas = map[string]Meta{}
	r.last = map[string]int64{}
	return nil
}

func validID(id string) bool {
	return id != "" && id == filepath.Base(id) && !strings.ContainsAny(id, `/\.`)
}

func dayOf(ms int64) string { return time.UnixMilli(ms).UTC().Format(dayLayout) }

// Record appends a value to a series, creating it if needed.
func (r *Recorder) Record(m Meta, t time.Time, v float64) error {
	if !validID(m.ID) {
		return fmt.Errorf("invalid series id %q", m.ID)
	}
	ms := t.UnixMilli()
	r.mu.Lock()
	defer r.mu.Unlock()
	if ms <= r.last[m.ID] {
		ms = r.last[m.ID] + 1 // keep strictly increasing even if the clock steps back
	}
	if old, ok := r.metas[m.ID]; !ok || old != m {
		if err := r.writeMetaLocked(m); err != nil {
			return err
		}
	}
	of, err := r.fileLocked(m.ID, dayOf(ms))
	if err != nil {
		return err
	}
	var buf [recordSize]byte
	binary.LittleEndian.PutUint64(buf[0:], uint64(ms))
	binary.LittleEndian.PutUint64(buf[8:], math.Float64bits(v))
	if _, err := of.w.Write(buf[:]); err != nil {
		return err
	}
	r.last[m.ID] = ms
	return nil
}

func (r *Recorder) writeMetaLocked(m Meta) error {
	dir := filepath.Join(r.root, m.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	tmp := filepath.Join(dir, "."+metaFile+".tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(dir, metaFile)); err != nil {
		return err
	}
	r.metas[m.ID] = m
	return nil
}

func (r *Recorder) fileLocked(id, day string) (*openFile, error) {
	if of, ok := r.files[id]; ok {
		if of.day == day {
			return of, nil
		}
		of.w.Flush()
		of.f.Close()
		delete(r.files, id)
	}
	path := filepath.Join(r.root, id, day+dataExt)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	// Trim a partial record left by a crash, then append.
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	size := st.Size() / recordSize * recordSize
	if size != st.Size() {
		if err := f.Truncate(size); err != nil {
			f.Close()
			return nil, err
		}
	}
	if size >= recordSize {
		var buf [8]byte
		if _, err := f.ReadAt(buf[:], size-recordSize); err == nil {
			if t := int64(binary.LittleEndian.Uint64(buf[:])); t > r.last[id] {
				r.last[id] = t
			}
		}
	}
	if _, err := f.Seek(size, io.SeekStart); err != nil {
		f.Close()
		return nil, err
	}
	of := &openFile{day: day, f: f, w: bufio.NewWriterSize(f, 16*1024)}
	r.files[id] = of
	return of, nil
}

// Flush writes buffered samples to disk.
func (r *Recorder) Flush() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var first error
	for _, of := range r.files {
		if err := of.w.Flush(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (r *Recorder) closeAllLocked() {
	for id, of := range r.files {
		of.w.Flush()
		of.f.Close()
		delete(r.files, id)
	}
}

// Close flushes and closes all files.
func (r *Recorder) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeAllLocked()
}

// closeSeriesLocked closes the open file of a series (before deleting it).
func (r *Recorder) closeSeriesLocked(id string) {
	if of, ok := r.files[id]; ok {
		of.w.Flush()
		of.f.Close()
		delete(r.files, id)
	}
}

func readMeta(dir string) (Meta, error) {
	b, err := os.ReadFile(filepath.Join(dir, metaFile))
	if err != nil {
		return Meta{}, err
	}
	var m Meta
	return m, json.Unmarshal(b, &m)
}

func dayFiles(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var days []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && strings.HasSuffix(n, dataExt) {
			if _, err := time.Parse(dayLayout, strings.TrimSuffix(n, dataExt)); err == nil {
				days = append(days, strings.TrimSuffix(n, dataExt))
			}
		}
	}
	sort.Strings(days)
	return days
}

func readRecordAt(f *os.File, i int64) (Sample, error) {
	var buf [recordSize]byte
	if _, err := f.ReadAt(buf[:], i*recordSize); err != nil {
		return Sample{}, err
	}
	return Sample{T: int64(binary.LittleEndian.Uint64(buf[0:])), V: math.Float64frombits(binary.LittleEndian.Uint64(buf[8:]))}, nil
}

// Series lists every recorded series with statistics, sorted by device
// then address.
func (r *Recorder) Series() []SeriesInfo {
	r.Flush()
	root := r.Root()
	entries, _ := os.ReadDir(root)
	out := []SeriesInfo{}
	for _, e := range entries {
		if !e.IsDir() || !validID(e.Name()) {
			continue
		}
		dir := filepath.Join(root, e.Name())
		m, err := readMeta(dir)
		if err != nil {
			continue
		}
		m.ID = e.Name()
		info := SeriesInfo{Meta: m}
		days := dayFiles(dir)
		for _, d := range days {
			f, err := os.Open(filepath.Join(dir, d+dataExt))
			if err != nil {
				continue
			}
			st, _ := f.Stat()
			n := st.Size() / recordSize
			info.Bytes += st.Size()
			info.Samples += n
			if n > 0 {
				if info.First == 0 {
					if s, err := readRecordAt(f, 0); err == nil {
						info.First = s.T
					}
				}
				if s, err := readRecordAt(f, n-1); err == nil {
					info.Last = s.T
				}
			}
			f.Close()
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Device() != b.Device() {
			return a.Device() < b.Device()
		}
		if a.Function != b.Function {
			return a.Function < b.Function
		}
		return a.Address < b.Address
	})
	return out
}

// Read returns all samples of a series with from <= t <= to (unix ms).
func (r *Recorder) Read(id string, from, to int64) ([]Sample, error) {
	if !validID(id) {
		return nil, fmt.Errorf("invalid series id %q", id)
	}
	r.Flush()
	dir := filepath.Join(r.Root(), id)
	var out []Sample
	for _, d := range dayFiles(dir) {
		dayStart, _ := time.Parse(dayLayout, d)
		ds := dayStart.UnixMilli()
		if ds > to || ds+86_400_000 <= from {
			continue
		}
		f, err := os.Open(filepath.Join(dir, d+dataExt))
		if err != nil {
			continue
		}
		st, _ := f.Stat()
		n := st.Size() / recordSize
		// First record with t >= from.
		start := int64(sort.Search(int(n), func(i int) bool {
			s, err := readRecordAt(f, int64(i))
			return err != nil || s.T >= from
		}))
		if start < n {
			if _, err := f.Seek(start*recordSize, io.SeekStart); err == nil {
				br := bufio.NewReaderSize(f, 64*1024)
				var buf [recordSize]byte
				for i := start; i < n; i++ {
					if _, err := io.ReadFull(br, buf[:]); err != nil {
						break
					}
					s := Sample{T: int64(binary.LittleEndian.Uint64(buf[0:])), V: math.Float64frombits(binary.LittleEndian.Uint64(buf[8:]))}
					if s.T > to {
						break
					}
					out = append(out, s)
				}
			}
		}
		f.Close()
	}
	return out, nil
}

// Query reads a range and reduces it to at most maxPoints samples by
// keeping the minimum and maximum of each time bucket, so spikes survive.
func (r *Recorder) Query(id string, from, to int64, maxPoints int) ([]Sample, error) {
	all, err := r.Read(id, from, to)
	if err != nil || maxPoints <= 0 || len(all) <= maxPoints {
		return all, err
	}
	buckets := maxPoints / 2
	if buckets < 1 {
		buckets = 1
	}
	width := float64(to-from+1) / float64(buckets)
	out := make([]Sample, 0, maxPoints)
	i := 0
	for b := 0; b < buckets && i < len(all); b++ {
		end := from + int64(math.Ceil(float64(b+1)*width))
		lo, hi := -1, -1
		for ; i < len(all) && all[i].T < end; i++ {
			if lo < 0 || all[i].V < all[lo].V {
				lo = i
			}
			if hi < 0 || all[i].V > all[hi].V {
				hi = i
			}
		}
		switch {
		case lo < 0:
		case lo == hi:
			out = append(out, all[lo])
		case lo < hi:
			out = append(out, all[lo], all[hi])
		default:
			out = append(out, all[hi], all[lo])
		}
	}
	return out, nil
}

// Usage returns the bytes used by recordings.
func (r *Recorder) Usage() int64 {
	r.Flush()
	var total int64
	_ = filepath.WalkDir(r.Root(), func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), dataExt) {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

type dayFile struct {
	id, day, path string
	size          int64
}

func (r *Recorder) allDayFiles() []dayFile {
	root := r.Root()
	var out []dayFile
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() || !validID(e.Name()) {
			continue
		}
		for _, d := range dayFiles(filepath.Join(root, e.Name())) {
			p := filepath.Join(root, e.Name(), d+dataExt)
			if st, err := os.Stat(p); err == nil {
				out = append(out, dayFile{e.Name(), d, p, st.Size()})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].day < out[j].day })
	return out
}

// Enforce deletes whole days older than retentionDays (0 = keep forever)
// and then the oldest days until usage is at most maxBytes (0 = no cap).
// The current day of a series is never deleted by the size cap. It
// returns the number of bytes freed.
func (r *Recorder) Enforce(retentionDays int, maxBytes int64, now time.Time) (int64, error) {
	r.Flush()
	files := r.allDayFiles()
	today := now.UTC().Format(dayLayout)
	var freed, total int64
	for _, f := range files {
		total += f.size
	}
	remove := func(f dayFile) {
		r.mu.Lock()
		if of, ok := r.files[f.id]; ok && of.day == f.day {
			r.closeSeriesLocked(f.id)
		}
		r.mu.Unlock()
		if os.Remove(f.path) == nil {
			freed += f.size
			total -= f.size
		}
	}
	if retentionDays > 0 {
		cutoff := now.UTC().AddDate(0, 0, -retentionDays).Format(dayLayout)
		for _, f := range files {
			if f.day < cutoff {
				remove(f)
			}
		}
		files = r.allDayFiles()
	}
	if maxBytes > 0 {
		for _, f := range files {
			if total <= maxBytes {
				break
			}
			if f.day == today {
				continue
			}
			remove(f)
		}
	}
	r.removeEmptySeries()
	return freed, nil
}

func (r *Recorder) removeEmptySeries() {
	root := r.Root()
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if e.IsDir() && validID(e.Name()) && len(dayFiles(filepath.Join(root, e.Name()))) == 0 {
			r.mu.Lock()
			r.closeSeriesLocked(e.Name())
			delete(r.metas, e.Name())
			r.mu.Unlock()
			os.RemoveAll(filepath.Join(root, e.Name()))
		}
	}
}

// DeleteSeries removes series entirely.
func (r *Recorder) DeleteSeries(ids []string) error {
	for _, id := range ids {
		if !validID(id) {
			return fmt.Errorf("invalid series id %q", id)
		}
		r.mu.Lock()
		r.closeSeriesLocked(id)
		delete(r.metas, id)
		delete(r.last, id)
		root := r.root
		r.mu.Unlock()
		if err := os.RemoveAll(filepath.Join(root, id)); err != nil {
			return err
		}
	}
	return nil
}

// DeleteRange removes samples with from <= t <= to from the given series
// (all series when ids is empty).
func (r *Recorder) DeleteRange(ids []string, from, to int64) error {
	if from > to {
		return errors.New("start is after end")
	}
	if len(ids) == 0 {
		for _, s := range r.Series() {
			ids = append(ids, s.ID)
		}
	}
	for _, id := range ids {
		if !validID(id) {
			return fmt.Errorf("invalid series id %q", id)
		}
		r.mu.Lock()
		r.closeSeriesLocked(id)
		root := r.root
		r.mu.Unlock()
		dir := filepath.Join(root, id)
		for _, d := range dayFiles(dir) {
			dayStart, _ := time.Parse(dayLayout, d)
			ds, de := dayStart.UnixMilli(), dayStart.UnixMilli()+86_399_999
			if de < from || ds > to {
				continue
			}
			path := filepath.Join(dir, d+dataExt)
			if from <= ds && to >= de {
				os.Remove(path)
				continue
			}
			if err := rewriteWithout(path, from, to); err != nil {
				return err
			}
		}
	}
	r.removeEmptySeries()
	return nil
}

func rewriteWithout(path string, from, to int64) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out := make([]byte, 0, len(b))
	for i := 0; i+recordSize <= len(b); i += recordSize {
		t := int64(binary.LittleEndian.Uint64(b[i:]))
		if t < from || t > to {
			out = append(out, b[i:i+recordSize]...)
		}
	}
	if len(out) == 0 {
		return os.Remove(path)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
