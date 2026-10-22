package printing

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/jung-kurt/gofpdf"
)

//go:embed DejaVuSans.ttf
var font []byte

type Line struct{ Name, Quantity, Price, Discount, Amount, Tax string }
type Invoice struct {
	Adjustments                                                                        string
	Logo                                                                               []byte
	LogoFormat                                                                         string
	TaxDocument                                                                        bool
	TaxSummary                                                                         string
	Title, Number, Business, Address, Party, PartyAddress, Date, DueDate, Notes, Total string
	Lines                                                                              []Line
}

func PDF(b Invoice) ([]byte, error) {
	p := gofpdf.New("P", "mm", "A4", "")
	p.SetMargins(16, 16, 16)
	p.SetAutoPageBreak(true, 18)
	p.AddUTF8FontFromBytes("local", "", font)
	p.SetFont("local", "", 11)
	p.SetFooterFunc(func() {
		p.SetY(-13)
		p.SetFont("local", "", 8)
		p.CellFormat(178, 5, fmt.Sprintf("Ledgerleaf · Page %d", p.PageNo()), "", 0, "C", false, 0, "")
	})
	p.AddPage()
	addLogo(p, b, 28)
	p.SetFont("local", "", 20)
	p.MultiCell(178, 10, b.Business, "", "L", false)
	p.SetFont("local", "", 10)
	p.MultiCell(178, 5, b.Address, "", "L", false)
	p.Ln(5)
	p.SetFont("local", "", 14)
	p.MultiCell(178, 8, b.Title+" · "+b.Number, "", "L", false)
	p.SetFont("local", "", 10)
	p.MultiCell(178, 6, "Date: "+b.Date+"   Due: "+b.DueDate, "", "L", false)
	p.Ln(4)
	p.MultiCell(178, 6, "Party: "+b.Party+"\n"+b.PartyAddress, "", "L", false)
	p.Ln(5)
	for _, l := range b.Lines {
		p.SetFillColor(241, 244, 238)
		p.MultiCell(178, 7, l.Name, "", "L", true)
		p.MultiCell(178, 6, "Quantity: "+l.Quantity+"  |  Unit price: INR "+l.Price+"  |  Discount: INR "+l.Discount, "", "L", false)
		p.MultiCell(178, 6, "Amount: INR "+l.Amount, "", "R", false)
		if l.Tax != "" {
			p.MultiCell(178, 5, l.Tax, "", "L", false)
		}
		p.Ln(3)
	}
	if b.TaxSummary != "" {
		p.MultiCell(178, 6, b.TaxSummary, "", "L", false)
	}
	p.SetFont("local", "", 14)
	if b.Adjustments != "" {
		p.MultiCell(178, 6, b.Adjustments, "", "L", false)
	}
	p.MultiCell(178, 9, "Total: INR "+b.Total, "T", "R", false)
	p.SetFont("local", "", 10)
	p.Ln(5)
	p.MultiCell(178, 6, b.Notes, "", "L", false)
	var out bytes.Buffer
	if e := p.Output(&out); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}

func addLogo(p *gofpdf.Fpdf, b Invoice, max float64) {
	if len(b.Logo) == 0 {
		return
	}
	opts := gofpdf.ImageOptions{ImageType: b.LogoFormat}
	info := p.RegisterImageOptionsReader("business-logo", opts, bytes.NewReader(b.Logo))
	if info == nil {
		return
	}
	w, h := info.Extent()
	if w <= 0 || h <= 0 {
		return
	}
	scale := max / w
	if h*scale > max {
		scale = max / h
	}
	w *= scale
	h *= scale
	p.ImageOptions("business-logo", p.GetX(), p.GetY(), w, h, false, opts, 0, "")
	p.SetY(p.GetY() + h + 3)
}
