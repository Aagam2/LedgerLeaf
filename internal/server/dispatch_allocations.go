package server

import (
	"billbook/internal/money"
	"database/sql"
	"encoding/json"
	"errors"
)

type dispatchBalance struct {
	ProductID int64  `json:"product_id"`
	Name      string `json:"name"`
	Unit      string `json:"unit"`
	Quantity  int64  `json:"quantity"`
	Used      int64  `json:"used"`
	Remaining int64  `json:"remaining"`
	Cost      int64  `json:"cost"`
	UsedCost  int64  `json:"used_cost"`
}

func dispatchBalances(tx *sql.Tx, b, id int64) (Bill, []dispatchBalance, error) {
	var raw string
	var d Bill
	if e := tx.QueryRow("SELECT snapshot FROM dispatches WHERE business_id=? AND id=?", b, id).Scan(&raw); e != nil {
		return d, nil, e
	}
	if e := json.Unmarshal([]byte(raw), &d); e != nil {
		return d, nil, e
	}
	balances := make([]dispatchBalance, 0, len(d.Lines))
	for _, l := range d.Lines {
		q, e := l.stockQuantity()
		if e != nil {
			return d, nil, e
		}
		row := dispatchBalance{ProductID: l.ProductID, Name: l.Name, Unit: l.baseUnit(), Quantity: q, Cost: l.Cost}
		if e = tx.QueryRow("SELECT COALESCE(SUM(quantity),0),COALESCE(SUM(value),0) FROM dispatch_allocations WHERE business_id=? AND dispatch_id=? AND product_id=?", b, id, l.ProductID).Scan(&row.Used, &row.UsedCost); e != nil {
			return d, nil, e
		}
		row.Remaining = q - row.Used
		if row.Remaining < 0 || row.UsedCost > row.Cost {
			return d, nil, errors.New("dispatch allocations exceed original quantities or cost")
		}
		balances = append(balances, row)
	}
	return d, balances, nil
}

// Cumulative allocation assigns the final paise exactly once, across both
// invoices and warehouse returns. Drafts do not reserve any quantities.
func dispatchAllocatedCost(row dispatchBalance, q int64) (int64, error) {
	if q <= 0 || q > row.Remaining {
		return 0, errors.New("quantity exceeds remaining dispatch; reload and review")
	}
	value, e := money.MulDiv(row.Cost, row.Used+q, row.Quantity)
	if e != nil {
		return 0, e
	}
	if value < row.UsedCost {
		return 0, errors.New("invalid dispatch cost allocation")
	}
	return value - row.UsedCost, nil
}

func recordDispatchInvoice(tx *sql.Tx, b int64, bill Bill, ref string) error {
	if len(bill.DispatchMatches) > 0 {
		for _, m := range bill.DispatchMatches {
			if _, e := tx.Exec("INSERT INTO dispatch_allocations(business_id,dispatch_id,product_id,reference,date,document_id,quantity,value) VALUES(?,?,?,?,?,?,?,?)", b, m.DispatchID, m.ProductID, ref, bill.Date, bill.ID, m.Quantity, m.Value); e != nil {
				return e
			}
		}
		return nil
	}
	for _, l := range bill.Lines {
		q, e := l.stockQuantity()
		if e != nil {
			return e
		}
		if _, e = tx.Exec("INSERT INTO dispatch_allocations(business_id,dispatch_id,product_id,reference,date,document_id,quantity,value) VALUES(?,?,?,?,?,?,?,?)", b, bill.DispatchID, l.ProductID, ref, bill.Date, bill.ID, q, l.Cost); e != nil {
			return e
		}
	}
	return nil
}

func dispatchAllocationDate(tx *sql.Tx, b, id int64, date string) error {
	var last sql.NullString
	if e := tx.QueryRow("SELECT MAX(date) FROM dispatch_allocations WHERE business_id=? AND dispatch_id=?", b, id).Scan(&last); e != nil {
		return e
	}
	if last.Valid && date < last.String {
		return errors.New("date predates an existing dispatch allocation")
	}
	return nil
}

// DispatchMatch quantities are base stock units. Values are assigned by the server.
type DispatchMatch struct {
	DispatchID int64 `json:"dispatch_id"`
	ProductID  int64 `json:"product_id"`
	Quantity   int64 `json:"quantity"`
	Value      int64 `json:"value,omitempty"`
}

func (bill Bill) hasDispatch() bool { return bill.DispatchID > 0 || len(bill.DispatchMatches) > 0 }

func validateDispatchMatches(tx *sql.Tx, b int64, bill *Bill) error {
	if bill.DispatchID != 0 || bill.Kind != "SALE" || bill.hasReceipt() || bill.SourceID != 0 || bill.DocumentType != "" {
		return errors.New("consolidated dispatch invoice must be a sale without other source links")
	}
	if len(bill.DispatchMatches) > 1000 {
		return errors.New("maximum 1,000 dispatch allocations per invoice")
	}
	byProduct := map[int64]*BillLine{}
	totals := map[int64]int64{}
	for i := range bill.Lines {
		l := &bill.Lines[i]
		l.Cost = 0
		byProduct[l.ProductID] = l
	}
	cache := map[int64]map[int64]dispatchBalance{}
	seen := map[[2]int64]bool{}
	for i := range bill.DispatchMatches {
		m := &bill.DispatchMatches[i]
		pair := [2]int64{m.DispatchID, m.ProductID}
		if m.DispatchID <= 0 || m.Quantity <= 0 || m.Quantity > 1000000000 || seen[pair] {
			return errors.New("invalid or duplicate dispatch allocation")
		}
		seen[pair] = true
		available, ok := cache[m.DispatchID]
		if !ok {
			d, balances, e := dispatchBalances(tx, b, m.DispatchID)
			if e != nil {
				return e
			}
			if bill.Date < d.Date || bill.PartyID != d.PartyID || bill.WarehouseID != d.WarehouseID {
				return errors.New("all dispatches must have the same customer and warehouse, and precede the invoice date")
			}
			if e = dispatchAllocationDate(tx, b, m.DispatchID, bill.Date); e != nil {
				return e
			}
			available = map[int64]dispatchBalance{}
			for _, row := range balances {
				available[row.ProductID] = row
			}
			cache[m.DispatchID] = available
		}
		row, ok := available[m.ProductID]
		line, exists := byProduct[m.ProductID]
		if !ok || !exists || row.Unit != line.baseUnit() {
			return errors.New("dispatch item or base unit differs from invoice")
		}
		value, e := dispatchAllocatedCost(row, m.Quantity)
		if e != nil {
			return e
		}
		m.Value = value
		line.Cost += value
		totals[m.ProductID] += m.Quantity
	}
	for _, l := range bill.Lines {
		q, e := l.stockQuantity()
		if e != nil {
			return e
		}
		if totals[l.ProductID] != q {
			return errors.New("dispatch allocations must exactly match every invoice line's base quantity")
		}
	}
	return nil
}
