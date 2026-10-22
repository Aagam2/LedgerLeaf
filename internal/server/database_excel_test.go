package server

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"testing"

	"billbook/internal/export"
)

func TestDatabaseExcelAllTablesAndLargeValues(t *testing.T) {
	s, call := fixture(t)
	payload := bytes.Repeat([]byte{0, 1, 255, 34}, 20000)
	if _, e := s.DB.Exec("INSERT INTO media_assets(business_id,format,content) VALUES(1,'png',?)", payload); e != nil {
		t.Fatal(e)
	}
	call("POST", "parties?business_id=2", `{"name":"=SUM(A1)","kind":"CUSTOMER","phone":"001234"}`, 200)
	book := call("GET", "database-excel", "", 200)
	z, e := zip.NewReader(strings.NewReader(book), int64(len(book)))
	if e != nil {
		t.Fatal(e)
	}
	var workbook struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
		} `xml:"sheets>sheet"`
	}
	contents := map[string][]byte{}
	for _, f := range z.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		data, e := io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
		contents[f.Name] = data
		d := xml.NewDecoder(bytes.NewReader(data))
		for {
			_, e = d.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				t.Fatal(f.Name, e)
			}
		}
	}
	if e = xml.Unmarshal(contents["xl/workbook.xml"], &workbook); e != nil {
		t.Fatal(e)
	}
	count := scalar(t, s.DB, "SELECT COUNT(*) FROM sqlite_schema WHERE type='table'")
	if len(workbook.Sheets) != int(count)+2 {
		t.Fatal("missing table sheets", len(workbook.Sheets), count)
	}
	var encoded strings.Builder
	foundParty, foundMedia := false, false
	for i, sheet := range workbook.Sheets {
		data := contents["xl/worksheets/sheet"+strconv.Itoa(i+1)+".xml"]
		if strings.Contains(string(data), "<f>") {
			t.Fatal("formula injected")
		}
		if strings.HasSuffix(sheet.Name, "_parties") {
			foundParty = strings.Contains(string(data), "=SUM(A1)") && strings.Contains(string(data), "001234")
		}
		if strings.HasSuffix(sheet.Name, "_media_assets") {
			foundMedia = strings.Contains(string(data), "@base64:")
		}
		if sheet.Name == "_LargeValues" {
			var rows struct {
				Rows []struct {
					Cells []struct {
						Text string `xml:"is>t"`
					} `xml:"c"`
				} `xml:"sheetData>row"`
			}
			if e = xml.Unmarshal(data, &rows); e != nil {
				t.Fatal(e)
			}
			for _, row := range rows.Rows[1:] {
				encoded.WriteString(row.Cells[2].Text)
			}
		}
	}
	decoded, e := base64.StdEncoding.DecodeString(encoded.String())
	if e != nil || !bytes.Equal(decoded, payload) {
		t.Fatal("attachment did not round trip", e)
	}
	if !foundParty || !foundMedia {
		t.Fatal("business or media missing")
	}
	call("POST", "database-excel", "", 405)
	assertIntegrity(t, s.DB)
}

func TestExcelTemplatesAndExportPermissions(t *testing.T) {
	s, call := fixture(t)
	for _, entity := range []string{"products", "parties"} {
		data := call("GET", "import-template?entity="+entity, "", 200)
		rows, e := export.ReadTable([]byte(data), "xlsx")
		if e != nil || len(rows) != 1 || rows[0][0] != "name" {
			t.Fatal(rows, e)
		}
	}
	call("GET", "import-template?entity=users", "", 400)
	owner := authClient(t, s)
	owner("POST", "auth/users", `{"username":"owner","password":"Test-Password-123!","role":"ADMIN","active":1}`, 200)
	for _, role := range []string{"ACCOUNTANT", "VIEWER", "BILLER"} {
		owner("POST", "auth/users", `{"username":"`+strings.ToLower(role)+`","password":"Test-Password-123!","role":"`+role+`","active":1}`, 200)
		user := authClient(t, s)
		user("POST", "auth/login", `{"username":"`+strings.ToLower(role)+`","password":"Test-Password-123!"}`, 200)
		user("GET", "database-excel", "", 403)
	}
	owner("GET", "database-excel", "", 200)
}
