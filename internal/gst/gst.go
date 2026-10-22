// Package gst performs exact, configured-rate GST calculations. Rates use basis
// points (1800 = 18%); money uses paise. It never infers a product's legal rate.
package gst

import (
	"billbook/internal/money"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var gstPattern = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$`)
var panPattern = regexp.MustCompile(`^[A-Z]{5}[0-9]{4}[A-Z]$`)

func State(s string) bool {
	n, e := strconv.Atoi(s)
	return e == nil && len(s) == 2 && n >= 1 && n <= 38
}
func PAN(s string) bool { return panPattern.MatchString(s) }
func GSTIN(s string) bool {
	if !gstPattern.MatchString(s) || !State(s[:2]) {
		return false
	}
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	sum, factor := 0, 2
	for i := 13; i >= 0; i-- {
		n := strings.IndexByte(alphabet, s[i]) * factor
		sum += n/36 + n%36
		factor = 3 - factor
	}
	return s[14] == alphabet[(36-sum%36)%36]
}

type Amounts struct{ Base, CGST, SGST, IGST, Cess, Total int64 }

func Calculate(net, rate, cess int64, inclusive, interstate bool) (Amounts, error) {
	var a Amounts
	if net < 0 || net > 1000000000000 || rate < 0 || rate > 10000 || rate%2 != 0 || cess < 0 || cess > 10000 {
		return a, errors.New("invalid tax amount or rate")
	}
	a.Base = net
	var e error
	if inclusive {
		a.Base, e = money.MulDiv(net, 10000, 10000+rate+cess)
		if e != nil {
			return a, e
		}
	}
	tax, e := money.MulDiv(a.Base, rate, 10000)
	if e != nil {
		return a, e
	}
	a.Cess, e = money.MulDiv(a.Base, cess, 10000)
	if e != nil {
		return a, e
	}
	if inclusive { // Allocate the final paise to GST (or cess) so gross stays exact.
		tax = net - a.Base - a.Cess
		if rate == 0 {
			a.Cess = net - a.Base
			tax = 0
		}
	}
	if interstate {
		a.IGST = tax
	} else {
		a.CGST = (tax + 1) / 2
		a.SGST = tax - a.CGST
	}
	a.Total = a.Base + tax + a.Cess
	return a, nil
}
