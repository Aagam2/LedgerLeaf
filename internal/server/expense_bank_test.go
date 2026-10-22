package server

import "testing"

func TestExpensesUseBusinessBankAccounts(t *testing.T) {
	s, call := fixture(t)
	call("POST", "banks?business_id=1", `{"code":"6100","name":"Trading bank"}`, 201)
	expense := `{"date":"2026-10-16","description":"Bank expense","amount":2500,"account":"6100","key":"bank-expense-request"}`
	call("POST", "expenses?business_id=2", expense, 400)
	call("POST", "expenses?business_id=1", expense, 200)
	call("POST", "expenses?business_id=1", expense, 200)
	if scalar(t, s.DB, "SELECT SUM(credit) FROM journal_lines WHERE business_id=1 AND account='6100'") != 2500 {
		t.Fatal("wrong or duplicate bank posting")
	}
	schedule := `{"description":"Rent","amount":1000,"account":"6100","next_date":"2026-10-16","interval_months":1,"active":1}`
	call("POST", "recurring-expenses?business_id=2", schedule, 400)
	call("POST", "recurring-expenses?business_id=1", schedule, 200)
	call("POST", "recurring-expense-post?business_id=1", `{"id":1,"next_date":"2026-10-16","key":"bank-recurring-post"}`, 200)
	if scalar(t, s.DB, "SELECT SUM(credit) FROM journal_lines WHERE business_id=1 AND account='6100'") != 3500 {
		t.Fatal("recurring payment used wrong bank")
	}
	call("POST", "expenses?business_id=1", `{"date":"2026-10-16","description":"Invalid","amount":100,"account":"5200","key":"invalid-expense-bank"}`, 400)
	assertIntegrity(t, s.DB)
}
