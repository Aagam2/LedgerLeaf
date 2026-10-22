package server

import (
	"strings"
	"testing"
)

func TestMasterImportAtomicAndXLSXRoundTrip(t *testing.T) {
	s, call := fixture(t)
	invalid := "name,kind,sku,unit,sale_price\nFirst,PRODUCT,SKU1,PCS,10000\nBad,PRODUCT,SKU2,PCS,notmoney\n"
	call("POST", "import-masters?business_id=1&entity=products&format=csv&key=master-import-request", invalid, 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM products") != 0 {
		t.Fatal("partial import")
	}
	valid := strings.Replace(invalid, "notmoney", "20000", 1)
	call("POST", "import-masters?business_id=1&entity=products&format=csv&key=master-import-request", valid, 200)
	call("POST", "import-masters?business_id=1&entity=products&format=csv&key=master-import-request", valid, 200)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM products") != 2 {
		t.Fatal("duplicate import")
	}
	xlsx := call("GET", "reports?business_id=1&kind=products&format=xlsx", "", 200)
	call("POST", "import-masters?business_id=2&entity=products&format=xlsx&key=xlsx-import-request", xlsx, 200)
	if scalar(t, s.DB, "SELECT SUM(sale_price) FROM products WHERE business_id=2") != 30000 {
		t.Fatal("xlsx round trip")
	}
	call("POST", "import-masters?business_id=2&entity=products&format=xlsx&key=duplicate-import-key", xlsx, 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM products WHERE business_id=2") != 2 {
		t.Fatal("duplicate SKU import changed data")
	}
	parties := "name,kind,phone,gstin,state_code\nBuyer,CUSTOMER,0012345,27AAPFU0939F1ZV,27\n"
	call("POST", "import-masters?business_id=1&entity=parties&format=csv&key=party-import-request", parties, 200)
	assertIntegrity(t, s.DB)
}
