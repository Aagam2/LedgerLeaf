package server

import (
	"billbook/internal/printing"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

func decimal(v int64, scale int) string {
	if v < 0 {
		return "-" + decimal(-v, scale)
	}
	div := int64(100)
	if scale == 3 {
		div = 1000
	}
	return fmt.Sprintf("%d.%0*d", v/div, scale, v%div)
}
func (s *Server) pdf(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	var snapshot string
	var reversed bool
	creditNote := r.URL.Path == "/api/credit-note-pdf"
	query := "SELECT snapshot,EXISTS(SELECT 1 FROM document_reversals v WHERE v.business_id=d.business_id AND v.document_id=d.id) FROM documents d WHERE business_id=? AND id=? AND status='FINALIZED'"
	if creditNote {
		query = "SELECT json_set(snapshot,'$.number',number,'$.date',date,'$.notes',reason||' (Original document ID: '||document_id||')'),0 FROM credit_notes WHERE business_id=? AND id=?"
	}
	if e = s.DB.QueryRow(query, b, r.URL.Query().Get("id")).Scan(&snapshot, &reversed); e != nil {
		fail(w, errors.New("finalized document not found"))
		return
	}
	var bill Bill
	if e = json.Unmarshal([]byte(snapshot), &bill); e != nil {
		fail(w, e)
		return
	}
	p := printing.Invoice{Title: bill.Kind, Number: bill.Number, Business: bill.BusinessName, Address: bill.BusinessAddress, Party: bill.PartyName, PartyAddress: bill.PartyAddress, Date: bill.Date, DueDate: bill.DueDate, Notes: bill.Notes + "\n" + bill.Terms + "\n" + bill.Footer + "\nBank: " + bill.BankDetails, Total: decimal(bill.Total, 2)}
	p.Adjustments = adjustmentSummary(bill)
	if e = s.invoiceLogo(b, bill, &p); e != nil {
		fail(w, e)
		return
	}
	if bill.TaxEnabled {
		p.Title = "TAX INVOICE - " + bill.Kind
		p.Address += "\nGSTIN: " + bill.BusinessGSTIN
		p.PartyAddress += "\nGSTIN: " + bill.PartyGSTIN + "\nPlace of supply: " + bill.PlaceOfSupply + "\nDelivery: " + bill.ShippingAddress
		p.TaxSummary = "Taxable / supply value: INR " + decimal(bill.baseTotal(), 2) + "\nGST and cess: INR " + decimal(bill.TaxTotal, 2)
		if bill.TaxMode == "REVERSE_CHARGE" {
			p.TaxSummary += "\nTax payable on reverse charge by recipient; not collected in invoice total."
		}
		if bill.TaxMode == "EXPORT_LUT" || bill.TaxMode == "SEZ_LUT" {
			p.Notes += "\nSupply meant for export / SEZ for authorised operations under bond or letter of undertaking without payment of integrated tax."
		}
		if bill.TaxMode == "EXPORT_TAX" || bill.TaxMode == "SEZ_TAX" {
			p.Notes += "\nSupply meant for export / SEZ for authorised operations on payment of integrated tax."
		}
		allExempt := true
		for _, l := range bill.Lines {
			if l.Treatment == "TAXABLE" {
				allExempt = false
			}
		}
		if allExempt && bill.Kind == "SALE" {
			p.Title = "BILL OF SUPPLY"
		}
	}
	if bill.TaxScheme == "COMPOSITION" && bill.Kind == "SALE" {
		p.Title = "BILL OF SUPPLY"
		p.Address += "\nGSTIN: " + bill.BusinessGSTIN
		p.Notes += "\nComposition taxable person, not eligible to collect tax on supplies."
	}
	if reversed {
		p.Title += " - RETURNED / CANCELLED"
	}
	if creditNote {
		p.Title = "CREDIT NOTE / RETURN - " + bill.Kind
	}
	for _, l := range bill.Lines {
		p.Lines = append(p.Lines, printing.Line{Name: l.Name, Quantity: decimal(l.Quantity, 3) + " " + l.Unit, Price: decimal(l.Price, 2), Discount: decimal(l.Discount, 2), Amount: decimal(l.Amount, 2), Tax: taxLineText(bill, l)})
	}
	var data []byte
	switch r.URL.Query().Get("paper") {
	case "":
		data, e = printing.PDF(p)
	case "58":
		data, e = printing.Thermal(p, 58)
	case "80":
		data, e = printing.Thermal(p, 80)
	default:
		fail(w, errors.New("unknown paper size"))
		return
	}
	if e != nil {
		fail(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Ledgerleaf-%d.pdf"`, bill.ID))
	w.Write(data)
}

func taxLineText(b Bill, l BillLine) string {
	if !b.TaxEnabled {
		return ""
	}
	return fmt.Sprintf("Quantity cess INR %s per %s\n", decimal(l.CessPerUnit, 2), l.baseUnit()) + fmt.Sprintf("HSN/SAC %s · %s · GST %s%% · Cess %s%%\nCGST %s · SGST/UTGST %s · IGST %s · Cess %s", l.HSN, l.Treatment, decimal(l.Rate, 2), decimal(l.CessRate, 2), decimal(l.CGST, 2), decimal(l.SGST, 2), decimal(l.IGST, 2), decimal(l.Cess, 2))
}
