// Package export writes spreadsheet files without external runtimes or formulas.
package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf16"
)

func Spreadsheet(w io.Writer, columns []string, next func() ([]interface{}, error)) error {
	return Workbook(w, []Sheet{{Name: "Report", Columns: columns, Next: next}})
}

// Sheet streams rows without loading an entire table into memory.
type Sheet struct {
	Name    string
	Columns []string
	Next    func() ([]interface{}, error)
}

func Workbook(w io.Writer, sheets []Sheet) error {
	z := zip.NewWriter(w)
	write := func(name, value string) error {
		f, err := z.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, value)
		return err
	}
	types := `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`
	book := `<?xml version="1.0"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>`
	rels := `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`
	seen := map[string]bool{}
	for i, s := range sheets {
		if s.Name == "" || len([]rune(s.Name)) > 31 || strings.ContainsAny(s.Name, "[]:*?/\\") || seen[strings.ToLower(s.Name)] {
			return fmt.Errorf("invalid or duplicate worksheet name %q", s.Name)
		}
		seen[strings.ToLower(s.Name)] = true
		var escaped bytes.Buffer
		if err := xml.EscapeText(&escaped, []byte(s.Name)); err != nil {
			return err
		}
		types += fmt.Sprintf(`<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i+1)
		book += fmt.Sprintf(`<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, escaped.String(), i+1, i+1)
		rels += fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, i+1)
	}
	for _, f := range [][2]string{{"[Content_Types].xml", types + `</Types>`}, {"_rels/.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`}, {"xl/workbook.xml", book + `</sheets></workbook>`}, {"xl/_rels/workbook.xml.rels", rels + `</Relationships>`}} {
		if err := write(f[0], f[1]); err != nil {
			return err
		}
	}
	for i, s := range sheets {
		if err := writeSheet(z, i+1, s.Columns, s.Next); err != nil {
			return fmt.Errorf("worksheet %s: %w", s.Name, err)
		}
	}
	return z.Close()
}

func writeSheet(z *zip.Writer, index int, columns []string, next func() ([]interface{}, error)) error {
	sheet, e := z.Create(fmt.Sprintf("xl/worksheets/sheet%d.xml", index))
	if e != nil {
		return e
	}
	if _, e = io.WriteString(sheet, `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`); e != nil {
		return e
	}
	rowNo := 0
	row := func(values []interface{}) error {
		if len(values) > 16384 {
			return fmt.Errorf("spreadsheet column limit exceeded")
		}
		rowNo++
		if rowNo > 1048576 {
			return fmt.Errorf("spreadsheet row limit exceeded; narrow the date range")
		}
		if _, e := fmt.Fprintf(sheet, `<row r="%d">`, rowNo); e != nil {
			return e
		}
		for i, v := range values {
			ref := column(i+1) + strconv.Itoa(rowNo)
			if n, ok := v.(int64); ok && n < 1000000000000000 && n > -1000000000000000 {
				if _, e := fmt.Fprintf(sheet, `<c r="%s"><v>%d</v></c>`, ref, n); e != nil {
					return e
				}
			} else {
				if _, e := fmt.Fprintf(sheet, `<c r="%s" t="inlineStr"><is><t xml:space="preserve">`, ref); e != nil {
					return e
				}
				value := ""
				if v != nil {
					value = fmt.Sprint(v)
					if b, ok := v.([]byte); ok {
						value = string(b)
					}
				}
				if len(utf16.Encode([]rune(value))) > 32767 {
					return fmt.Errorf("spreadsheet cell limit exceeded")
				}
				if e := xml.EscapeText(sheet, []byte(value)); e != nil {
					return e
				}
				if _, e := io.WriteString(sheet, `</t></is></c>`); e != nil {
					return e
				}
			}
		}
		_, e := io.WriteString(sheet, `</row>`)
		return e
	}
	headers := make([]interface{}, len(columns))
	for i, c := range columns {
		headers[i] = c
	}
	if e = row(headers); e != nil {
		return e
	}
	for {
		values, e := next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		if e = row(values); e != nil {
			return e
		}
	}
	if _, e = io.WriteString(sheet, `</sheetData></worksheet>`); e != nil {
		return e
	}
	return nil
}
func column(n int) string {
	out := ""
	for n > 0 {
		n--
		out = string(rune('A'+n%26)) + out
		n /= 26
	}
	return out
}
