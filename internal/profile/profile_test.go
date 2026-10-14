package profile

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"modbusstudio/internal/datafmt"
	"modbusstudio/internal/engine"
)

const meterCSV = "\xef\xbb\xbfEnergy meter register map;;;;;\n" +
	"Parameter;Address;Data Type;Multiplier;Unit;Remarks\n" +
	"Voltage V1N;30001;Float;;V;\n" +
	"Voltage V2N;30003;Float;;V;\n" +
	"Frequency;30071;UINT16;x0.01;Hz;\n" +
	";;;;;\n" +
	"Total kWh;30101;UINT32 CDAB;1/10;kWh;rolls over\n" +
	"CT ratio;40001;INT16;;;R/W\n" +
	"bogus;abc;;;;\n"

func TestImportCSVModicon(t *testing.T) {
	rows, err := parseDelimited([]byte(meterCSV))
	if err != nil {
		t.Fatal(err)
	}
	hdr, m := DetectHeader(rows)
	if hdr != 1 || m[FieldName] != 0 || m[FieldAddress] != 1 || m[FieldFormat] != 2 || m[FieldGain] != 3 || m[FieldUnit] != 4 || m[FieldNote] != 5 {
		t.Fatalf("header %d mapping %v", hdr, m)
	}
	opt := ImportOptions{HeaderRow: hdr, Mapping: m, ByteOrder: "abcd"}
	if mode := SuggestAddressMode(rows, opt); mode != AddrModicon {
		t.Fatalf("mode %s", mode)
	}
	opt.AddressMode = AddrModicon
	p, warn := BuildProfile(rows, opt)
	if len(warn) != 1 || !strings.Contains(warn[0], "row 9") {
		t.Fatalf("warnings %v", warn)
	}
	p.Name = "Test meter"
	if err := p.Normalize(); err != nil {
		t.Fatal(err)
	}
	byName := map[string]Register{}
	for _, r := range p.Registers {
		byName[r.Name] = r
	}
	if r := byName["Voltage V1N"]; r.Function != 4 || r.Address != 0 || r.Format != "float_abcd" || r.Unit != "V" {
		t.Errorf("V1N %+v", r)
	}
	if r := byName["Frequency"]; r.Address != 70 || r.Gain != 0.01 || r.Format != "unsigned" {
		t.Errorf("freq %+v", r)
	}
	if r := byName["Total kWh"]; r.Format != "ulong_cdab" || r.Gain != 0.1 || r.Note != "rolls over" {
		t.Errorf("kWh %+v", r)
	}
	if r := byName["CT ratio"]; r.Function != 3 || r.Address != 0 || r.Format != "signed" {
		t.Errorf("CT %+v", r)
	}

	polls, err := ToPolls(p, "c1", 5, 1000)
	if err != nil {
		t.Fatal(err)
	}
	// Holding 0; input 0-3 (gap to 70 is too big) ; input 70-71? no: 70 alone, 100-101.
	var got []string
	for _, pc := range polls {
		got = append(got, pc.Name)
	}
	want := []string{"Test meter #5 · Holding 0–0", "Test meter #5 · Input regs 0–3", "Test meter #5 · Input regs 70–70", "Test meter #5 · Input regs 100–101"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("polls\n got %v\nwant %v", got, want)
	}
	if pc := polls[1]; pc.Format != "float_abcd" || pc.Tags[2].Name != "Voltage V2N" || pc.SlaveID != 5 {
		t.Fatalf("poll %+v", pc)
	}
	if pc := polls[3]; pc.Tags[100].Gain != 0.1 {
		t.Fatalf("kWh poll %+v", pc)
	}
}

func makeXLSX(t *testing.T) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	put := func(name, body string) {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	put("xl/workbook.xml", `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Map" sheetId="1" r:id="rId1"/></sheets></workbook>`)
	put("xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="worksheets/sheet1.xml"/></Relationships>`)
	put("xl/sharedStrings.xml", `<sst><si><t>Address</t></si><si><t>Name</t></si><si><t>Type</t></si><si><r><t>Active </t></r><r><t>power</t></r></si><si><t>float cdab</t></si></sst>`)
	put("xl/worksheets/sheet1.xml", `<worksheet><sheetData>
<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c><c r="C1" t="s"><v>2</v></c></row>
<row r="3"><c r="A3"><v>3060</v></c><c r="B3" t="s"><v>3</v></c><c r="C3" t="s"><v>4</v></c></row>
<row r="4"><c r="A4"><v>3062</v></c><c r="B4" t="inlineStr"><is><t>Reactive power</t></is></c><c r="D4"><v>9</v></c></row>
</sheetData></worksheet>`)
	zw.Close()
	return buf.Bytes()
}

func TestImportXLSX(t *testing.T) {
	path := filepath.Join(t.TempDir(), "map.xlsx")
	os.WriteFile(path, makeXLSX(t), 0o644)
	rows, err := ReadTable(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || rows[2][1] != "Active power" || rows[3][1] != "Reactive power" || rows[3][3] != "9" || len(rows[1]) != 0 {
		t.Fatalf("rows %q", rows)
	}
	hdr, m := DetectHeader(rows)
	p, warn := BuildProfile(rows, ImportOptions{HeaderRow: hdr, Mapping: m, AddressMode: AddrZero, DefaultFunction: 3, ByteOrder: "abcd"})
	if len(warn) != 0 || len(p.Registers) != 2 || p.Registers[0].Format != "float_cdab" || p.Registers[0].Address != 3060 || p.Registers[1].Format != "unsigned" {
		t.Fatalf("%+v %v", p.Registers, warn)
	}
}

func TestParsers(t *testing.T) {
	for in, want := range map[string]float64{"0.1": 0.1, "x0.01": 0.01, "×10": 10, "1/100": 0.01, "/10": 0.1, "÷1000": 0.001, "": 1} {
		if g, ok := parseGain(in); !ok || g != want {
			t.Errorf("gain %q = %v", in, g)
		}
	}
	for in, want := range map[string]string{
		"Float": "float_abcd", "IEEE754 float word-swapped": "float_cdab", "INT32": "long_abcd", "Unsigned Long DCBA": "ulong_dcba",
		"UINT16": "unsigned", "int16": "signed", "Double": "double_abcd", "BOOL": "bit", "S64": "long64_abcd", "DWORD": "ulong_abcd",
	} {
		if f, _, ok := parseFormat(in, "abcd"); !ok || string(f) != want {
			t.Errorf("format %q = %s", in, f)
		}
	}
	for in, want := range map[string]int{"3": 3, "FC04": 4, "Holding Register": 3, "Input register": 4, "Coil": 1, "Discrete input": 2, "4x": 3} {
		if v, ok := parseFunction(in); !ok || v != want {
			t.Errorf("function %q = %d", in, v)
		}
	}
	if v, err := parseInt("0x0BB8"); err != nil || v != 3000 {
		t.Errorf("hex %d %v", v, err)
	}
}

func TestLibraryAndFromPoll(t *testing.T) {
	lib := NewLibrary(t.TempDir(), "")
	list := lib.List()
	if len(list) == 0 || list[0].Source != Bundled || list[0].Error != "" {
		t.Fatalf("bundled %+v", list)
	}
	demo, err := lib.Get(list[0].ID)
	if err != nil || len(demo.Registers) < 10 {
		t.Fatalf("demo %v", err)
	}
	if _, err := lib.Save(demo, Bundled, ""); err == nil {
		t.Fatal("bundled must be read-only")
	}
	if _, err := lib.Save(demo, Shared, ""); err == nil {
		t.Fatal("no shared dir configured")
	}

	poll := engine.PollConfig{SlaveID: 4, Function: 3, Address: 10, Quantity: 5, Format: "signed",
		Tags: map[int]engine.Tag{12: {Name: "Power", Gain: 0.1, Unit: "kW", Decimals: -1}}}
	poll.CellFormats = map[int]datafmt.Format{11: "float_abcd"}
	p := FromPoll(poll, "From window")
	// 10 signed, 11-12 float (tag at 12 is inside the float -> ignored), 13, 14
	if len(p.Registers) != 4 || p.Registers[1].Format != "float_abcd" || p.Registers[1].Address != 11 {
		t.Fatalf("%+v", p.Registers)
	}
	id, err := lib.Save(p, User, "")
	if err != nil {
		t.Fatal(err)
	}
	id2, _ := lib.Save(p, User, "") // second copy gets a new file
	if id == id2 {
		t.Fatal("copy overwrote original")
	}
	if id3, _ := lib.Save(p, User, id); id3 != id {
		t.Fatal("update should keep the id")
	}
	if n := len(lib.List()); n != len(list)+2 {
		t.Fatalf("list %d", n)
	}
	if err := lib.Delete(id); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Get("user:../../etc/passwd"); err == nil {
		t.Fatal("path traversal accepted")
	}
}
