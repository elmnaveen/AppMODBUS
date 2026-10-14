package profile

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"appmodbus/internal/datafmt"
)

// Field is a profile column that can be mapped from a spreadsheet.
type Field string

const (
	FieldAddress  Field = "address"
	FieldFunction Field = "function"
	FieldName     Field = "name"
	FieldFormat   Field = "format"
	FieldGain     Field = "gain"
	FieldOffset   Field = "offset"
	FieldUnit     Field = "unit"
	FieldDecimals Field = "decimals"
	FieldNote     Field = "note"
)

// Fields lists every mappable field in display order.
var Fields = []Field{FieldAddress, FieldFunction, FieldName, FieldFormat, FieldGain, FieldOffset, FieldUnit, FieldDecimals, FieldNote}

// AddressMode tells how spreadsheet addresses are numbered.
type AddressMode string

const (
	AddrZero    AddressMode = "zero"    // protocol addresses, 0-based
	AddrOne     AddressMode = "one"     // register numbers, 1-based
	AddrModicon AddressMode = "modicon" // 0xxxx/1xxxx/3xxxx/4xxxx: the prefix selects the function
)

// ImportOptions control how rows become registers.
type ImportOptions struct {
	HeaderRow       int           `json:"headerRow"` // 0-based; data starts on the next row
	Mapping         map[Field]int `json:"mapping"`   // column index, or -1/absent
	AddressMode     AddressMode   `json:"addressMode"`
	DefaultFunction int           `json:"defaultFunction"` // used when no function column
	ByteOrder       datafmt.Order `json:"byteOrder"`       // for 32/64-bit types without an explicit order
}

// ReadTable reads the first sheet of a .csv, .tsv, .txt or .xlsx file.
func ReadTable(path string) ([][]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".xlsx", ".xlsm":
		return readXLSX(b)
	case ".xls":
		return nil, fmt.Errorf("old .xls files are not supported; save the sheet as .xlsx or .csv")
	}
	return parseDelimited(b)
}

func parseDelimited(b []byte) ([][]string, error) {
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM from Excel
	if !utf8.Valid(b) {
		b = latin1ToUTF8(b)
	}
	first := string(b)
	if i := strings.IndexAny(first, "\r\n"); i >= 0 {
		first = first[:i]
	}
	// Pick the delimiter that splits the first line into the most columns.
	delim, best := ',', 0
	for _, d := range []rune{',', ';', '\t', '|'} {
		if n := strings.Count(first, string(d)); n > best {
			delim, best = d, n
		}
	}
	r := csv.NewReader(bytes.NewReader(b))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	return r.ReadAll()
}

func latin1ToUTF8(b []byte) []byte {
	out := make([]rune, len(b))
	for i, c := range b {
		out[i] = rune(c)
	}
	return []byte(string(out))
}

func norm(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.NewReplacer("_", " ", "-", " ", ".", " ", "(", " ", ")", " ").Replace(s)), " "))
}

// columnScore says how well a header cell describes a field (0 = not at all).
func columnScore(f Field, header string) int {
	h := norm(header)
	has := func(words ...string) bool {
		for _, w := range words {
			if h == w || strings.HasPrefix(h, w+" ") || strings.HasSuffix(h, " "+w) || strings.Contains(h, " "+w+" ") {
				return true
			}
		}
		return false
	}
	contains := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(h, w) {
				return true
			}
		}
		return false
	}
	switch f {
	case FieldFunction:
		if contains("function", "register type", "object type", "data area") || has("fc", "table", "area", "type of register") {
			return 3
		}
	case FieldFormat:
		if contains("data type", "datatype", "format", "data format", "value type") {
			return 3
		}
		if has("type", "size", "length") && !contains("register") {
			return 2
		}
	case FieldAddress:
		if contains("address", "addr") {
			return 3
		}
		if has("register", "reg", "register no", "reg no", "start", "modbus") && !contains("type") {
			return 2
		}
		if h == "offset" || h == "no" || h == "#" {
			return 1
		}
	case FieldGain:
		if contains("gain", "scale", "factor", "multiplier", "multiply", "resolution") {
			return 3
		}
	case FieldOffset:
		if h == "offset" || contains("offset value", "add") {
			return 2
		}
	case FieldUnit:
		if has("unit", "units", "uom", "eng unit", "engineering unit") || contains("unit") {
			return 3
		}
	case FieldDecimals:
		if contains("decimal", "precision", "dp") {
			return 3
		}
	case FieldName:
		if has("name", "parameter", "parameters", "tag", "signal", "measurement", "quantity", "variable", "point") || contains("parameter", "name") {
			return 3
		}
		if contains("description", "desc") {
			return 2
		}
	case FieldNote:
		if contains("note", "remark", "comment") {
			return 3
		}
		if contains("description", "desc") {
			return 1
		}
	}
	return 0
}

// DetectHeader finds the header row (the first row naming at least two
// fields) and maps columns to fields.
func DetectHeader(rows [][]string) (headerRow int, mapping map[Field]int) {
	bestRow, bestMap, bestN := 0, map[Field]int{}, 0
	for i := 0; i < len(rows) && i < 20; i++ {
		m := mapColumns(rows[i])
		if len(m) > bestN {
			bestRow, bestMap, bestN = i, m, len(m)
		}
		if bestN >= 3 {
			break
		}
	}
	return bestRow, bestMap
}

func mapColumns(header []string) map[Field]int {
	m := map[Field]int{}
	used := map[int]bool{}
	for _, f := range Fields {
		best, bestScore := -1, 0
		for c, h := range header {
			if used[c] {
				continue
			}
			if s := columnScore(f, h); s > bestScore {
				best, bestScore = c, s
			}
		}
		if best >= 0 {
			// Address and offset can both claim a column titled "Offset";
			// keep it as the address unless another address column exists.
			m[f], used[best] = best, true
		}
	}
	if _, ok := m[FieldAddress]; !ok {
		if c, ok := m[FieldOffset]; ok {
			m[FieldAddress] = c
			delete(m, FieldOffset)
		}
	}
	return m
}

// SuggestAddressMode guesses Modicon numbering when every address falls in
// the 0xxxx/1xxxx/3xxxx/4xxxx ranges and there is no function column.
func SuggestAddressMode(rows [][]string, opt ImportOptions) AddressMode {
	col, ok := opt.Mapping[FieldAddress]
	if !ok || col < 0 {
		return AddrZero
	}
	_, hasFn := opt.Mapping[FieldFunction]
	n, modicon := 0, 0
	for _, row := range rows[min(opt.HeaderRow+1, len(rows)):] {
		if col >= len(row) {
			continue
		}
		v, err := parseInt(row[col])
		if err != nil {
			continue
		}
		n++
		if (v >= 10001 && v <= 19999) || (v >= 30001 && v <= 49999 && v != 40000) ||
			(v >= 100001 && v <= 165536) || (v >= 300001 && v <= 465536) {
			modicon++
		}
	}
	if n > 0 && modicon == n && !hasFn {
		return AddrModicon
	}
	return AddrZero
}

func parseInt(s string) (int, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, " ", ""))
	if strings.HasPrefix(strings.ToLower(s), "0x") {
		v, err := strconv.ParseInt(s[2:], 16, 32)
		return int(v), err
	}
	if strings.HasSuffix(strings.ToLower(s), "h") && len(s) > 1 {
		if v, err := strconv.ParseInt(s[:len(s)-1], 16, 32); err == nil {
			return int(v), nil
		}
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && f == math.Trunc(f) {
		return int(f), nil
	}
	return 0, fmt.Errorf("%q is not a number", s)
}

func parseFunction(s string) (int, bool) {
	t := norm(s)
	t = strings.TrimPrefix(t, "fc")
	t = strings.TrimSpace(t)
	if v, err := strconv.Atoi(t); err == nil && v >= 1 && v <= 4 {
		return v, true
	}
	switch {
	case strings.Contains(t, "holding") || t == "hr" || t == "4x" || t == "r/w" || t == "rw" || strings.HasPrefix(t, "4"):
		return 3, true
	case strings.Contains(t, "input reg") || t == "ir" || t == "3x" || strings.HasPrefix(t, "3"):
		return 4, true
	case strings.Contains(t, "coil") || t == "0x" || t == "co":
		return 1, true
	case strings.Contains(t, "discrete") || strings.Contains(t, "input status") || t == "di" || t == "1x":
		return 2, true
	}
	return 0, false
}

// parseFormat maps a data-type cell ("Float", "INT32 CDAB", "UINT16", …).
func parseFormat(s string, def datafmt.Order) (datafmt.Format, bool, bool) {
	t := strings.ToLower(strings.NewReplacer(" ", "", "_", "", "-", "", "/", "").Replace(s))
	if t == "" {
		return datafmt.Unsigned, false, true
	}
	order := def
	for _, o := range []datafmt.Order{datafmt.CDAB, datafmt.BADC, datafmt.DCBA, datafmt.ABCD} {
		if strings.Contains(t, string(o)) {
			order = o
		}
	}
	if strings.Contains(t, "swap") || strings.Contains(t, "inverse") || strings.Contains(t, "littleendianword") {
		order = datafmt.CDAB
	}
	if strings.Contains(t, "littleendian") && !strings.Contains(t, "word") {
		order = datafmt.DCBA
	}
	pick := func(kind string) datafmt.Format { return datafmt.Format(kind + "_" + string(order)) }
	switch {
	case strings.Contains(t, "bit") || strings.Contains(t, "bool") || strings.Contains(t, "coil"):
		return "bit", true, true
	case strings.Contains(t, "double") || strings.Contains(t, "float64") || strings.Contains(t, "real64") || strings.Contains(t, "lreal"):
		return pick("double"), false, true
	case strings.Contains(t, "float") || strings.Contains(t, "real") || strings.Contains(t, "ieee") || t == "f32" || t == "fp32":
		return pick("float"), false, true
	case strings.Contains(t, "64"):
		return pick("long64"), false, true
	case strings.Contains(t, "uint32") || strings.Contains(t, "u32") || strings.Contains(t, "ulong") || strings.Contains(t, "unsigned32") || strings.Contains(t, "unsignedlong") || strings.Contains(t, "dword") || strings.Contains(t, "udint"):
		return pick("ulong"), false, true
	case strings.Contains(t, "32") || strings.Contains(t, "long") || strings.Contains(t, "dint"):
		return pick("long"), false, true
	case strings.Contains(t, "hex"):
		return datafmt.Hex, false, true
	case strings.Contains(t, "uint") || strings.Contains(t, "u16") || strings.Contains(t, "unsigned") || strings.Contains(t, "word"):
		return datafmt.Unsigned, false, true
	case strings.Contains(t, "int") || strings.Contains(t, "s16") || strings.Contains(t, "signed") || strings.Contains(t, "short"):
		return datafmt.Signed, false, true
	}
	return datafmt.Unsigned, false, false
}

// parseGain accepts "0.1", "x0.01", "×10", "1/10", "/100" and "÷10".
func parseGain(s string) (float64, bool) {
	t := strings.TrimSpace(strings.NewReplacer("×", "", "*", "", " ", "").Replace(strings.ToLower(s)))
	t = strings.TrimPrefix(t, "x")
	if t == "" {
		return 1, true
	}
	div := false
	for _, p := range []string{"1/", "/", "÷"} {
		if strings.HasPrefix(t, p) {
			t, div = strings.TrimPrefix(t, p), true
			break
		}
	}
	v, err := strconv.ParseFloat(t, 64)
	if err != nil || v == 0 {
		return 1, false
	}
	if div {
		v = 1 / v
	}
	return v, true
}

// BuildProfile converts table rows into registers. Rows without a usable
// address are skipped and reported as warnings.
func BuildProfile(rows [][]string, opt ImportOptions) (Profile, []string) {
	var p Profile
	var warnings []string
	cell := func(row []string, f Field) string {
		c, ok := opt.Mapping[f]
		if !ok || c < 0 || c >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[c])
	}
	if opt.DefaultFunction < 1 || opt.DefaultFunction > 4 {
		opt.DefaultFunction = 3
	}
	if opt.ByteOrder == "" {
		opt.ByteOrder = datafmt.ABCD
	}
	seen := map[[2]int]bool{}
	for i := opt.HeaderRow + 1; i < len(rows); i++ {
		row := rows[i]
		line := i + 1
		addrText := cell(row, FieldAddress)
		if addrText == "" {
			continue // blank or section-title row
		}
		addr, err := parseInt(addrText)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("row %d: %v", line, err))
			continue
		}
		r := Register{Function: opt.DefaultFunction, Gain: 1, Decimals: -1}
		if fn := cell(row, FieldFunction); fn != "" {
			if v, ok := parseFunction(fn); ok {
				r.Function = v
			} else {
				warnings = append(warnings, fmt.Sprintf("row %d: unknown function %q, using %02d", line, fn, opt.DefaultFunction))
			}
		}
		switch opt.AddressMode {
		case AddrOne:
			addr--
		case AddrModicon:
			prefix, rest := addr/10000, addr%10000
			if addr >= 100000 {
				prefix, rest = addr/100000, addr%100000
			}
			fn := map[int]int{0: 1, 1: 2, 3: 4, 4: 3}[prefix]
			if fn == 0 || rest == 0 {
				warnings = append(warnings, fmt.Sprintf("row %d: %d is not a Modicon address", line, addr))
				continue
			}
			r.Function, addr = fn, rest-1
		}
		r.Address = addr
		f, isBit, ok := parseFormat(cell(row, FieldFormat), opt.ByteOrder)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("row %d: unknown data type %q, using Unsigned", line, cell(row, FieldFormat)))
		}
		if isBit && r.Function > 2 {
			r.Function = 1
		}
		r.Format = f
		r.Name = cell(row, FieldName)
		r.Unit = cell(row, FieldUnit)
		r.Note = cell(row, FieldNote)
		if g := cell(row, FieldGain); g != "" {
			if v, ok := parseGain(g); ok {
				r.Gain = v
			} else {
				warnings = append(warnings, fmt.Sprintf("row %d: gain %q not understood, using 1", line, g))
			}
		}
		if o := cell(row, FieldOffset); o != "" {
			if v, err := strconv.ParseFloat(o, 64); err == nil {
				r.Offset = v
			}
		}
		if d := cell(row, FieldDecimals); d != "" {
			if v, err := strconv.Atoi(d); err == nil && v >= 0 && v <= 9 {
				r.Decimals = v
			}
		}
		if r.Address < 0 || r.Address+r.Span() > 0x10000 {
			warnings = append(warnings, fmt.Sprintf("row %d: address %d out of range", line, r.Address))
			continue
		}
		key := [2]int{r.Function, r.Address}
		if seen[key] {
			warnings = append(warnings, fmt.Sprintf("row %d: duplicate address %d skipped", line, r.Address))
			continue
		}
		seen[key] = true
		p.Registers = append(p.Registers, r)
	}
	return p, warnings
}
