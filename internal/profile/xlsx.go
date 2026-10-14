package profile

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"
)

// readXLSX returns the cell text of the first worksheet of an .xlsx file.
// It supports shared strings, inline strings, numbers and booleans, which
// is all a register map needs (formulas yield their cached value).
func readXLSX(b []byte) ([][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, errors.New("not a valid .xlsx file")
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	read := func(name string) ([]byte, error) {
		f, ok := files[name]
		if !ok {
			return nil, errors.New("missing " + name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(io.LimitReader(rc, 64<<20))
	}

	sheet, err := firstSheetPath(read)
	if err != nil {
		return nil, err
	}
	var shared []string
	if data, err := read("xl/sharedStrings.xml"); err == nil {
		shared = parseSharedStrings(data)
	}
	data, err := read(sheet)
	if err != nil {
		return nil, err
	}
	return parseSheet(data, shared)
}

func firstSheetPath(read func(string) ([]byte, error)) (string, error) {
	wb, err := read("xl/workbook.xml")
	if err != nil {
		return "", err
	}
	var w struct {
		Sheets []struct {
			ID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.Unmarshal(wb, &w); err != nil || len(w.Sheets) == 0 {
		return "", errors.New("workbook has no sheets")
	}
	rels, err := read("xl/_rels/workbook.xml.rels")
	if err != nil {
		return "xl/worksheets/sheet1.xml", nil
	}
	var r struct {
		Rels []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(rels, &r); err != nil {
		return "", err
	}
	for _, rel := range r.Rels {
		if rel.ID == w.Sheets[0].ID {
			if strings.HasPrefix(rel.Target, "/") {
				return strings.TrimPrefix(rel.Target, "/"), nil
			}
			return path.Join("xl", rel.Target), nil
		}
	}
	return "xl/worksheets/sheet1.xml", nil
}

type richText struct {
	T  string `xml:"t"`
	Rs []struct {
		T string `xml:"t"`
	} `xml:"r"`
}

func (r richText) text() string {
	if len(r.Rs) == 0 {
		return r.T
	}
	var b strings.Builder
	for _, x := range r.Rs {
		b.WriteString(x.T)
	}
	return b.String()
}

func parseSharedStrings(data []byte) []string {
	var sst struct {
		SI []richText `xml:"si"`
	}
	_ = xml.Unmarshal(data, &sst)
	out := make([]string, len(sst.SI))
	for i, si := range sst.SI {
		out[i] = si.text()
	}
	return out
}

// colIndex converts the letters of a cell reference ("BC12") to 0-based.
func colIndex(ref string) int {
	n := 0
	for _, c := range ref {
		if c < 'A' || c > 'Z' {
			break
		}
		n = n*26 + int(c-'A'+1)
	}
	return n - 1
}

func parseSheet(data []byte, shared []string) ([][]string, error) {
	var ws struct {
		Rows []struct {
			R     int `xml:"r,attr"`
			Cells []struct {
				Ref string   `xml:"r,attr"`
				T   string   `xml:"t,attr"`
				V   string   `xml:"v"`
				IS  richText `xml:"is"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal(data, &ws); err != nil {
		return nil, err
	}
	byRow := map[int][]string{}
	maxRow := 0
	for i, row := range ws.Rows {
		r := row.R
		if r == 0 {
			r = i + 1
		}
		var cells []string
		for j, c := range row.Cells {
			col := j
			if c.Ref != "" {
				col = colIndex(c.Ref)
			}
			if col < 0 || col > 1000 {
				continue
			}
			for len(cells) <= col {
				cells = append(cells, "")
			}
			v := c.V
			switch c.T {
			case "s":
				if k, err := strconv.Atoi(v); err == nil && k >= 0 && k < len(shared) {
					v = shared[k]
				}
			case "inlineStr":
				v = c.IS.text()
			case "b":
				if v == "1" {
					v = "TRUE"
				} else {
					v = "FALSE"
				}
			}
			cells[col] = v
		}
		byRow[r] = cells
		if r > maxRow {
			maxRow = r
		}
	}
	out := make([][]string, 0, maxRow)
	// Keep blank rows so row numbers in warnings match the spreadsheet.
	for r := 1; r <= maxRow && r <= 100000; r++ {
		out = append(out, byRow[r])
	}
	return out, nil
}
