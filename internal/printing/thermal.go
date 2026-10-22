package printing

import (
	"bytes"
	"fmt"
	"github.com/jung-kurt/gofpdf"
)

func Thermal(b Invoice, width float64) ([]byte, error) {
	if width != 58 && width != 80 {
		return nil, fmt.Errorf("paper width must be 58 or 80 mm")
	}
	p := gofpdf.NewCustom(&gofpdf.InitType{OrientationStr: "P", UnitStr: "mm", Size: gofpdf.SizeType{Wd: width, Ht: 220}})
	p.SetMargins(3, 4, 3)
	p.SetAutoPageBreak(true, 5)
	p.AddUTF8FontFromBytes("local", "", font)
	p.SetFont("local", "", 8)
	p.AddPage()
	addLogo(p, b, 18)
	cell := func(v string) { p.MultiCell(width-6, 4, v, "", "L", false) }
	cell(b.Business)
	cell(b.Address)
	cell(b.Title + " " + b.Number)
	cell(b.Date)
	cell("Customer: " + b.Party)
	cell(b.PartyAddress)
	for _, l := range b.Lines {
		cell("----------------")
		cell(l.Name)
		cell(l.Quantity + " x " + l.Price)
		cell("Discount " + l.Discount + " Amount " + l.Amount)
		if l.Tax != "" {
			cell(l.Tax)
		}
	}
	cell(b.TaxSummary)
	cell(b.Adjustments)
	p.SetFont("local", "", 11)
	cell("Total INR " + b.Total)
	p.SetFont("local", "", 8)
	cell(b.Notes)
	var out bytes.Buffer
	if e := p.Output(&out); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}
