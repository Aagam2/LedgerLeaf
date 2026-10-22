package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestSpreadsheetPreservesPrecisionAndEscapesFormulas(t *testing.T) {
	var out bytes.Buffer
	called := false
	e := Spreadsheet(&out, []string{"name", "amount", "huge"}, func() ([]interface{}, error) {
		if called {
			return nil, io.EOF
		}
		called = true
		return []interface{}{"=SUM(A1)&<", int64(123), int64(9007199254740993)}, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	z, e := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range z.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		data, e := io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
		d := xml.NewDecoder(bytes.NewReader(data))
		for {
			_, e = d.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				t.Fatal(f.Name, e)
			}
		}
		if f.Name == "xl/worksheets/sheet1.xml" {
			s := string(data)
			if strings.Contains(s, "<f>") || !strings.Contains(s, "9007199254740993") || !strings.Contains(s, "<v>123</v>") || !strings.Contains(s, "=SUM(A1)&amp;&lt;") {
				t.Fatal(s)
			}
		}
	}
}

func TestReadSpreadsheetRejectsFormulasAndKeepsEmptyCells(t *testing.T) {
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	f, _ := z.Create("xl/worksheets/sheet1.xml")
	io.WriteString(f, `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>name</t></is></c><c r="C1" t="inlineStr"><is><t>sku</t></is></c></row><row><c r="A2"><f>1+1</f><v>2</v></c></row></sheetData></worksheet>`)
	z.Close()
	if _, e := ReadTable(out.Bytes(), "xlsx"); e == nil {
		t.Fatal("formula accepted")
	}
	out.Reset()
	called := false
	if e := Spreadsheet(&out, []string{"name", "phone", "sku"}, func() ([]interface{}, error) {
		if called {
			return nil, io.EOF
		}
		called = true
		return []interface{}{"Tea", "", "001"}, nil
	}); e != nil {
		t.Fatal(e)
	}
	rows, e := ReadTable(out.Bytes(), "xlsx")
	if e != nil || len(rows) != 2 || rows[1][1] != "" || rows[1][2] != "001" {
		t.Fatal(rows, e)
	}
}
