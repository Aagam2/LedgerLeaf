package export

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type richText struct {
	Text string `xml:"t"`
	Runs []struct {
		Text string `xml:"t"`
	} `xml:"r"`
}

func (r richText) text() string {
	s := r.Text
	for _, x := range r.Runs {
		s += x.Text
	}
	return s
}

// ReadTable accepts a plain CSV or the first XLSX worksheet. Formula cells are
// rejected rather than evaluating cached results. Archive and row limits bound memory.
func ReadTable(data []byte, format string) ([][]string, error) {
	if format == "csv" {
		r := csv.NewReader(bytes.NewReader(data))
		r.FieldsPerRecord = -1
		out := [][]string{}
		for {
			row, e := r.Read()
			if e == io.EOF {
				break
			}
			if e != nil {
				return nil, e
			}
			out = append(out, row)
			if len(out) > 10001 {
				return nil, errors.New("maximum 10,000 data rows per import")
			}
		}
		return out, nil
	}
	if format != "xlsx" {
		return nil, errors.New("use CSV or XLSX")
	}
	z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		return nil, e
	}
	if len(z.File) > 100 {
		return nil, errors.New("too many archive entries")
	}
	var total uint64
	files := map[string]*zip.File{}
	for _, f := range z.File {
		total += f.UncompressedSize64
		if f.UncompressedSize64 > 40<<20 || total > 40<<20 {
			return nil, errors.New("expanded spreadsheet exceeds 40 MB")
		}
		files[f.Name] = f
	}
	read := func(name string) ([]byte, error) {
		f := files[name]
		if f == nil {
			return nil, errors.New("worksheet not found")
		}
		r, e := f.Open()
		if e != nil {
			return nil, e
		}
		defer r.Close()
		b, e := io.ReadAll(io.LimitReader(r, (40<<20)+1))
		if len(b) > 40<<20 {
			return nil, errors.New("worksheet exceeds limit")
		}
		return b, e
	}
	shared := []string{}
	if files["xl/sharedStrings.xml"] != nil {
		b, e := read("xl/sharedStrings.xml")
		if e != nil {
			return nil, e
		}
		var ss struct {
			Items []richText `xml:"si"`
		}
		if e = xml.Unmarshal(b, &ss); e != nil {
			return nil, e
		}
		for _, s := range ss.Items {
			shared = append(shared, s.text())
		}
	}
	b, e := read("xl/worksheets/sheet1.xml")
	if e != nil {
		return nil, e
	}
	d := xml.NewDecoder(bytes.NewReader(b))
	out := [][]string{}
	for {
		tok, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "row" {
			continue
		}
		var row struct {
			Cells []struct {
				Ref     string   `xml:"r,attr"`
				Type    string   `xml:"t,attr"`
				Value   string   `xml:"v"`
				Formula *string  `xml:"f"`
				Inline  richText `xml:"is"`
			} `xml:"c"`
		}
		if e = d.DecodeElement(&row, &start); e != nil {
			return nil, e
		}
		record := []string{}
		for i, c := range row.Cells {
			if c.Formula != nil {
				return nil, errors.New("formula cells are not allowed in imports")
			}
			col := 0
			for _, ch := range c.Ref {
				if ch < 'A' || ch > 'Z' {
					break
				}
				col = col*26 + int(ch-'A'+1)
			}
			if col == 0 {
				col = i + 1
			}
			if col > 64 {
				return nil, errors.New("maximum 64 columns")
			}
			for len(record) < col {
				record = append(record, "")
			}
			value := c.Value
			if c.Type == "inlineStr" {
				value = c.Inline.text()
			}
			if c.Type == "s" {
				n, e := strconv.Atoi(c.Value)
				if e != nil || n < 0 || n >= len(shared) {
					return nil, fmt.Errorf("invalid shared string")
				}
				value = shared[n]
			}
			record[col-1] = value
		}
		if strings.TrimSpace(strings.Join(record, "")) != "" {
			out = append(out, record)
		}
		if len(out) > 10001 {
			return nil, errors.New("maximum 10,000 data rows per import")
		}
	}
	return out, nil
}
