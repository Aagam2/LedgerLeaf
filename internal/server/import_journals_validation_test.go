package server

import (
	"fmt"
	"strings"
	"testing"
)

func journalRows() [][]string {
	return [][]string{{"reference", "date", "description", "account", "debit", "credit"}, {"opening", "2026-04-01", "Opening cash", "1000", "10000", "0"}, {"opening", "2026-04-01", "Opening cash", "3000", "0", "10000"}}
}

func TestParseJournalRowsValidation(t *testing.T) {
	cases := []struct {
		name, want string
		change     func([][]string) [][]string
	}{
		{"unbalanced", "unbalanced", func(r [][]string) [][]string { r[2][5] = "9999"; return r }},
		{"invalid date", "invalid fields", func(r [][]string) [][]string { r[1][1] = "2026-02-30"; return r }},
		{"different dates", "differ within reference", func(r [][]string) [][]string { r[2][1] = "2026-04-02"; return r }},
		{"different descriptions", "differ within reference", func(r [][]string) [][]string { r[2][2] = "Other"; return r }},
		{"control account", "operational control", func(r [][]string) [][]string { r[1][3] = "1200"; return r }},
		{"two positive sides", "invalid fields", func(r [][]string) [][]string { r[1][5] = "1"; return r }},
		{"zero line", "invalid fields", func(r [][]string) [][]string { r[1][4] = "0"; return r }},
		{"fractional paise", "invalid fields", func(r [][]string) [][]string { r[1][4] = "1.5"; return r }},
		{"overflow", "invalid fields", func(r [][]string) [][]string { r[1][4] = "99999999999999999999999999"; return r }},
		{"duplicate header", "duplicate header", func(r [][]string) [][]string { r[0][5] = "debit"; return r }},
		{"unknown header", "missing column", func(r [][]string) [][]string { r[0][5] = "amount"; return r }},
		{"short row", "wrong column count", func(r [][]string) [][]string { r[1] = r[1][:5]; return r }},
		{"single-line journal", "at least two lines", func(r [][]string) [][]string { r[2][0] = "other"; return r }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			entries, e := parseJournalRows(c.change(journalRows()))
			if e == nil || !strings.Contains(e.Error(), c.want) || entries != nil {
				t.Fatalf("entries=%v error=%v; want %s", entries, e, c.want)
			}
		})
	}
}

func TestParseJournalRowsOrderingAndLimits(t *testing.T) {
	r := journalRows()
	r[0][0] = "\ufeffreference"
	r = append(r, []string{"expense", "2026-04-02", "Rent", "5200", "2500", "0"}, []string{"expense", "2026-04-02", "Rent", "1000", "0", "2500"})
	entries, e := parseJournalRows(r)
	if e != nil || len(entries) != 2 || entries[0].Reference != "opening" || entries[1].Reference != "expense" {
		t.Fatalf("%v %v", entries, e)
	}
	r = journalRows()
	r[1][4] = "1000000000000"
	r[2][5] = "1000000000000"
	if _, e = parseJournalRows(r); e != nil {
		t.Fatal(e)
	}
	r = append(r, []string{"opening", "2026-04-01", "Opening cash", "1000", "1", "0"}, []string{"opening", "2026-04-01", "Opening cash", "3000", "0", "1"})
	if _, e = parseJournalRows(r); e == nil || !strings.Contains(e.Error(), "total exceeds") {
		t.Fatal(e)
	}
	r = journalRows()[:1]
	for i := 0; i < 1001; i++ {
		ref := fmt.Sprintf("ref-%d", i)
		r = append(r, []string{ref, "2026-04-01", "Opening", "1000", "1", "0"}, []string{ref, "2026-04-01", "Opening", "3000", "0", "1"})
	}
	if _, e = parseJournalRows(r); e == nil || !strings.Contains(e.Error(), "1,000 journals") {
		t.Fatal(e)
	}
	r = journalRows()
	for i := 0; i < 99; i++ {
		r = append(r, []string{"opening", "2026-04-01", "Opening cash", "1000", "1", "0"})
	}
	if _, e = parseJournalRows(r); e == nil || !strings.Contains(e.Error(), "100 lines") {
		t.Fatal(e)
	}
}
