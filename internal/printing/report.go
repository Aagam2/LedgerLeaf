package printing

import (
	"bytes"
	"fmt"
	"github.com/jung-kurt/gofpdf"
	"io"
	"strings"
)

func Report(title string, columns []string, next func() ([]interface{}, error)) ([]byte, error) {
	p := gofpdf.New("P", "mm", "A4", "")
	p.SetMargins(14, 14, 14)
	p.SetAutoPageBreak(true, 16)
	p.AddUTF8FontFromBytes("local", "", font)
	p.SetFont("local", "", 10)
	p.SetFooterFunc(func() {
		p.SetY(-12)
		p.CellFormat(182, 5, fmt.Sprintf("Ledgerleaf · Page %d", p.PageNo()), "", 0, "C", false, 0, "")
	})
	p.AddPage()
	p.SetFont("local", "", 15)
	p.MultiCell(182, 8, title, "", "L", false)
	p.SetFont("local", "", 9)
	p.MultiCell(182, 5, "Amounts are integer paise; quantities are thousandths. Columns retain export precision.", "", "L", false)
	for count := 0; ; count++ {
		values, e := next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		if count >= 25000 {
			return nil, fmt.Errorf("PDF is limited to 25,000 rows; narrow dates or export XLSX/CSV")
		}
		parts := []string{}
		for i, c := range columns {
			v := ""
			if values[i] != nil {
				v = fmt.Sprint(values[i])
				if b, ok := values[i].([]byte); ok {
					v = string(b)
				}
			}
			parts = append(parts, strings.ReplaceAll(c, "_", " ")+": "+v)
		}
		p.Ln(2)
		p.MultiCell(182, 5, strings.Join(parts, "  |  "), "B", "L", false)
	}
	var out bytes.Buffer
	if e := p.Output(&out); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}
