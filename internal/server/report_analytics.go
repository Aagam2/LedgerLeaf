package server

// Aggregate each document before grouping so invoice counts do not depend on
// the number of lines. Returns belong to their own posting period.
const salesActivity = `WITH events AS (
 SELECT d.party_id,d.date,d.snapshot,1 AS sign,1 AS invoices,0 AS returns FROM documents d WHERE d.business_id=? AND d.kind='SALE' AND d.status='FINALIZED' AND d.date BETWEEN ? AND ?
 UNION ALL SELECT d.party_id,c.date,c.snapshot,-1,0,1 FROM credit_notes c JOIN documents d ON d.business_id=c.business_id AND d.id=c.document_id WHERE c.business_id=? AND d.kind='SALE' AND c.date BETWEEN ? AND ?
), totals AS (
 SELECT party_id,date,sign,invoices,returns,
 COALESCE((SELECT SUM(COALESCE(json_extract(l.value,'$.amount'),0)) FROM json_each(events.snapshot,'$.lines') l),0) AS net_sales,
 COALESCE((SELECT SUM(COALESCE(json_extract(l.value,'$.cost'),0)) FROM json_each(events.snapshot,'$.lines') l),0) AS cost_of_sales
 FROM events
) `

func salesActivityQuery(kind string) string {
	if kind == "monthly-sales" {
		return salesActivity + `SELECT substr(date,1,7) AS month,SUM(invoices) AS invoices,SUM(returns) AS credit_notes,SUM(sign*net_sales) AS net_sales,SUM(sign*cost_of_sales) AS cost_of_sales,SUM(sign*(net_sales-cost_of_sales)) AS gross_profit FROM totals GROUP BY month ORDER BY month`
	}
	return salesActivity + `SELECT p.id AS party_id,p.name AS customer,SUM(t.invoices) AS invoices,SUM(t.returns) AS credit_notes,SUM(t.sign*t.net_sales) AS net_sales,SUM(t.sign*t.cost_of_sales) AS cost_of_sales,SUM(t.sign*(t.net_sales-t.cost_of_sales)) AS gross_profit FROM totals t JOIN parties p ON p.id=t.party_id AND p.business_id=? GROUP BY p.id,p.name ORDER BY net_sales DESC,p.id`
}

// Both invoice and credit-note dates are filtered independently. Quantities
// are normalized using frozen conversion factors, never today's product data.
const postedReportLines = `WITH posted AS (
 SELECT d.kind,d.snapshot,1 AS sign FROM documents d WHERE d.business_id=? AND d.status='FINALIZED' AND d.date BETWEEN ? AND ?
 UNION ALL SELECT d.kind,c.snapshot,-1 FROM credit_notes c JOIN documents d ON d.business_id=c.business_id AND d.id=c.document_id WHERE c.business_id=? AND c.date BETWEEN ? AND ?
), lines AS (
 SELECT p.kind,p.sign,COALESCE(json_extract(p.snapshot,'$.tax_enabled'),0) AS tax_enabled,
 COALESCE(json_extract(p.snapshot,'$.tax_mode'),'NORMAL') AS tax_mode,
 json_extract(l.value,'$.product_id') AS product_id,json_extract(l.value,'$.name') AS product,json_extract(l.value,'$.sku') AS sku,
 COALESCE(NULLIF(json_extract(l.value,'$.base_unit'),''),json_extract(l.value,'$.unit')) AS unit,
 json_extract(l.value,'$.quantity')*COALESCE(NULLIF(json_extract(l.value,'$.unit_factor'),0),1000)/1000 AS quantity,
 COALESCE(json_extract(l.value,'$.hsn'),'') AS hsn,COALESCE(json_extract(l.value,'$.treatment'),'NON_GST') AS treatment,
 COALESCE(json_extract(l.value,'$.rate'),0) AS rate,json_extract(l.value,'$.amount') AS amount,COALESCE(json_extract(l.value,'$.cost'),0) AS cost,
 COALESCE(json_extract(l.value,'$.cgst'),0) AS cgst,COALESCE(json_extract(l.value,'$.sgst'),0) AS sgst,
 COALESCE(json_extract(l.value,'$.igst'),0) AS igst,COALESCE(json_extract(l.value,'$.cess'),0) AS cess
 FROM posted p,json_each(p.snapshot,'$.lines') l
) `

func analyticsQuery(kind string) string {
	if kind == "hsn-summary" {
		return postedReportLines + `SELECT kind,hsn,unit,treatment,tax_mode,rate,SUM(sign*quantity) AS quantity,SUM(sign*amount) AS taxable_value,SUM(sign*cgst) AS cgst,SUM(sign*sgst) AS sgst,SUM(sign*igst) AS igst,SUM(sign*cess) AS cess FROM lines WHERE tax_enabled=1 GROUP BY kind,hsn,unit,treatment,tax_mode,rate ORDER BY kind,hsn,unit,treatment,tax_mode,rate`
	}
	return postedReportLines + `SELECT product_id,sku,product,unit,SUM(sign*quantity) AS quantity,SUM(sign*amount) AS net_sales,SUM(sign*cost) AS cost_of_sales,SUM(sign*(amount-cost)) AS gross_profit FROM lines WHERE kind='SALE' GROUP BY product_id,sku,product,unit ORDER BY net_sales DESC,product_id,sku,product,unit`
}
