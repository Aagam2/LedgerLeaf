// Package money uses integer minor units and scaled integer quantities only.
package money

import (
	"errors"
	"math/big"
)

// MulDiv rounds nonnegative rational values half up, rejecting int64 overflow.
func MulDiv(a, b, d int64) (int64, error) {
	if a < 0 || b < 0 || d <= 0 {
		return 0, errors.New("invalid amount")
	}
	n := new(big.Int).Mul(big.NewInt(a), big.NewInt(b))
	n.Add(n, big.NewInt(d/2))
	n.Quo(n, big.NewInt(d))
	if !n.IsInt64() {
		return 0, errors.New("amount overflow")
	}
	return n.Int64(), nil
}
