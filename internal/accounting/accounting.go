package accounting

import (
	"database/sql"
	"errors"
	"fmt"
)

type Line struct {
	Account       string
	Debit, Credit int64
}

func Seed(tx *sql.Tx, b int64) error {
	for _, a := range [][3]string{{"2050", "Goods received, invoice pending", "LIABILITY"}, {"1400", "Supplier advances", "ASSET"}, {"2200", "Customer advances", "LIABILITY"}, {"1300", "Input CGST", "ASSET"}, {"1310", "Input SGST / UTGST", "ASSET"}, {"1320", "Input IGST", "ASSET"}, {"1330", "Input cess", "ASSET"}, {"2100", "Output CGST / RCM", "LIABILITY"}, {"2110", "Output SGST / UTGST / RCM", "LIABILITY"}, {"2120", "Output IGST / RCM", "LIABILITY"}, {"2130", "Output cess / RCM", "LIABILITY"}, {"1000", "Cash", "ASSET"}, {"1010", "Bank", "ASSET"}, {"1100", "Accounts receivable", "ASSET"}, {"1200", "Inventory", "ASSET"}, {"1250", "Goods dispatched, invoice pending", "ASSET"}, {"2000", "Accounts payable", "LIABILITY"}, {"3000", "Opening equity", "EQUITY"}, {"3100", "Retained earnings", "EQUITY"}, {"4000", "Sales revenue", "INCOME"}, {"5000", "Cost of goods sold", "EXPENSE"}, {"5100", "Stock adjustments", "EXPENSE"}, {"5200", "Expenses", "EXPENSE"}, {"5300", "Invoice rounding", "EXPENSE"}} {
		if _, e := tx.Exec("INSERT OR IGNORE INTO accounts(business_id,code,name,kind) VALUES(?,?,?,?)", b, a[0], a[1], a[2]); e != nil {
			return e
		}
	}
	_, e := tx.Exec("INSERT OR IGNORE INTO bank_accounts(business_id,code,name) VALUES(?,'1010','Bank')", b)
	return e
}
func Post(tx *sql.Tx, b int64, date, ref, description string, lines []Line) error {
	var debit, credit int64
	for _, l := range lines {
		if l.Debit < 0 || l.Credit < 0 || l.Debit > 1000000000000 || l.Credit > 1000000000000 || (l.Debit != 0 && l.Credit != 0) {
			return errors.New("invalid journal line")
		}
		debit += l.Debit
		credit += l.Credit
	}
	if debit != credit {
		return fmt.Errorf("unbalanced journal: debit %d credit %d", debit, credit)
	}
	if debit == 0 {
		return nil
	}
	res, e := tx.Exec("INSERT INTO journals(business_id,date,reference,description) VALUES(?,?,?,?)", b, date, ref, description)
	if e != nil {
		return e
	}
	id, e := res.LastInsertId()
	if e != nil {
		return e
	}
	for _, l := range lines {
		if l.Debit+l.Credit == 0 {
			continue
		}
		if _, e = tx.Exec("INSERT INTO journal_lines(business_id,journal_id,account,debit,credit) VALUES(?,?,?,?,?)", b, id, l.Account, l.Debit, l.Credit); e != nil {
			return e
		}
	}
	return nil
}
