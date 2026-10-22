package server

import (
	"billbook/internal/printing"
	"net/http"
)

// Image export uses the same prepared, frozen print content as PDF. It remains
// behind each existing document endpoint's authentication and business scoping.
func imageExportData(w http.ResponseWriter, r *http.Request, invoice printing.Invoice) bool {
	if r.URL.Query().Get("format") != "image-data" {
		return false
	}
	respond(w, http.StatusOK, invoice)
	return true
}
