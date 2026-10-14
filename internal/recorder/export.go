package recorder

import (
	"archive/zip"
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MaxExcelRows is the row limit of an .xlsx sheet, minus the header.
const MaxExcelRows = 1_048_575

// table merges series into rows keyed by timestamp. Values recorded by the
// same poll share a timestamp and so share a row.
type table struct {
	headers []string
	times   []int64
	rows    map[int64][]string
}

func (r *Recorder) buildTable(ids []string, from, to int64) (*table, error) {
	if len(ids) == 0 {
		return nil, errors.New("select at least one series to export")
	}
	metas := map[string]Meta{}
	for _, s := range r.Series() {
		metas[s.ID] = s.Meta
	}
	t := &table{headers: []string{"Time"}, rows: map[int64][]string{}}
	for col, id := range ids {
		m, ok := metas[id]
		if !ok {
			return nil, fmt.Errorf("unknown series %s", id)
		}
		h := m.Label()
		if m.Unit != "" {
			h += " [" + m.Unit + "]"
		}
		t.headers = append(t.headers, fmt.Sprintf("%s (%s, %s %d)", h, m.Device(), fnCode(m.Function), m.Address))
		digits := significantDigits(m.Format)
		samples, err := r.Read(id, from, to)
		if err != nil {
			return nil, err
		}
		for _, s := range samples {
			row, ok := t.rows[s.T]
			if !ok {
				row = make([]string, len(ids))
				t.rows[s.T] = row
				t.times = append(t.times, s.T)
			}
			row[col] = formatValue(s.V, digits)
		}
	}
	sort.Slice(t.times, func(i, j int) bool { return t.times[i] < t.times[j] })
	return t, nil
}

// significantDigits is the precision a value of the format really has:
// 32-bit floats carry about 7 digits; integers scaled by a gain are
// rounded to 12 to drop binary noise (0.1 × 2297 = 229.70000000000002).
func significantDigits(format string) int {
	if strings.HasPrefix(format, "float_") {
		return 7
	}
	return 12
}

func formatValue(v float64, digits int) string {
	r, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'g', digits, 64), 64)
	return strconv.FormatFloat(r, 'f', -1, 64)
}

func fnCode(fc int) string { return fmt.Sprintf("FC%02d", fc) }

func formatTime(ms int64, loc *time.Location) string {
	return time.UnixMilli(ms).In(loc).Format("2006-01-02 15:04:05.000")
}

// ExportCSV writes the selected series to a CSV file and returns the
// number of data rows.
func (r *Recorder) ExportCSV(path string, ids []string, from, to int64, loc *time.Location) (int, error) {
	t, err := r.buildTable(ids, from, to)
	if err != nil {
		return 0, err
	}
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	bw := bufio.NewWriter(f)
	bw.WriteString("\xef\xbb\xbf") // BOM so Excel opens UTF-8 (°C, ×) correctly
	w := csv.NewWriter(bw)
	w.Write(t.headers)
	for _, ts := range t.times {
		w.Write(append([]string{formatTime(ts, loc)}, t.rows[ts]...))
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return 0, err
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		return 0, err
	}
	return len(t.times), f.Close()
}

// ExportXLSX writes the selected series to an Excel workbook.
func (r *Recorder) ExportXLSX(path string, ids []string, from, to int64, loc *time.Location) (int, error) {
	t, err := r.buildTable(ids, from, to)
	if err != nil {
		return 0, err
	}
	if len(t.times) > MaxExcelRows {
		return 0, fmt.Errorf("%d rows exceed Excel's limit of %d; export a shorter range or use CSV", len(t.times), MaxExcelRows)
	}
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	if err := writeXLSX(f, t, loc); err != nil {
		f.Close()
		os.Remove(path)
		return 0, err
	}
	return len(t.times), f.Close()
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

func colName(i int) string {
	s := ""
	for i++; i > 0; i = (i - 1) / 26 {
		s = string(rune('A'+(i-1)%26)) + s
	}
	return s
}

// writeXLSX produces a minimal single-sheet workbook. Times are written as
// Excel date serials with a date-time number format, values as numbers.
func writeXLSX(out io.Writer, t *table, loc *time.Location) error {
	zw := zip.NewWriter(out)
	put := func(name, body string) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, body)
		return err
	}
	parts := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Recording" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/styles.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><numFmts count="1"><numFmt numFmtId="164" formatCode="yyyy-mm-dd hh:mm:ss.000"/></numFmts><fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf/></cellStyleXfs><cellXfs count="3"><xf/><xf numFmtId="164" applyNumberFormat="1"/><xf fontId="1" applyFont="1"/></cellXfs></styleSheet>`,
	}
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml"} {
		if err := put(name, parts[name]); err != nil {
			return err
		}
	}
	w, err := zw.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(w, 256*1024)
	bw.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>` +
		`<cols><col min="1" max="1" width="24" customWidth="1"/></cols><sheetData>`)
	bw.WriteString(`<row r="1">`)
	for c, h := range t.headers {
		fmt.Fprintf(bw, `<c r="%s1" t="inlineStr" s="2"><is><t>%s</t></is></c>`, colName(c), xmlEscape(h))
	}
	bw.WriteString(`</row>`)
	excelEpoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	for i, ts := range t.times {
		rn := i + 2
		tm := time.UnixMilli(ts).In(loc)
		_, off := tm.Zone()
		serial := float64(tm.UnixMilli()+int64(off)*1000-excelEpoch.UnixMilli()) / 86_400_000
		fmt.Fprintf(bw, `<row r="%d"><c r="A%d" s="1"><v>%s</v></c>`, rn, rn, strconv.FormatFloat(serial, 'f', 10, 64))
		for c, v := range t.rows[ts] {
			if v != "" {
				fmt.Fprintf(bw, `<c r="%s%d"><v>%s</v></c>`, colName(c+1), rn, v)
			}
		}
		bw.WriteString(`</row>`)
	}
	bw.WriteString(`</sheetData></worksheet>`)
	if err := bw.Flush(); err != nil {
		return err
	}
	return zw.Close()
}

// DefaultRoot returns the platform's usual place for large per-user data:
// %LOCALAPPDATA% on Windows, ~/Library/Application Support on macOS and
// $XDG_DATA_HOME (~/.local/share) elsewhere.
func DefaultRoot() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return filepath.Join(d, "AppMODBUS", "recordings")
	}
	home, _ := os.UserHomeDir()
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "AppMODBUS", "recordings")
	}
	if _, err := os.Stat(filepath.Join(home, "Library")); err == nil {
		return filepath.Join(home, "Library", "Application Support", "AppMODBUS", "recordings")
	}
	return filepath.Join(home, ".local", "share", "AppMODBUS", "recordings")
}
