package server

// Return metadata comes from the original frozen invoice; amounts come from
// the credit note. A later change to party registration cannot reclassify it.
const gstPreparationLines = `WITH events AS (
 SELECT d.id,d.kind,d.date AS invoice_date,d.date,d.number,d.number AS invoice,d.snapshot AS original,d.snapshot AS amounts,1 AS sign,'INVOICE' AS entry FROM documents d WHERE d.business_id=? AND d.status='FINALIZED' AND d.date BETWEEN ? AND ?
 UNION ALL SELECT d.id,d.kind,d.date,c.date,c.number,d.number,d.snapshot,c.snapshot,-1,'CREDIT_NOTE' FROM credit_notes c JOIN documents d ON d.business_id=c.business_id AND d.id=c.document_id WHERE c.business_id=? AND c.date BETWEEN ? AND ?
), lines AS (
 SELECT e.*,COALESCE(json_extract(original,'$.party_gstin'),'') AS gstin,COALESCE(json_extract(original,'$.party_name'),'') AS party,
 COALESCE(json_extract(original,'$.place_of_supply'),'') AS place_of_supply,COALESCE(json_extract(original,'$.supplier_state'),'') AS supplier_state,
 COALESCE(json_extract(original,'$.tax_mode'),'NORMAL') AS tax_mode,COALESCE(json_extract(original,'$.tax_enabled'),0) AS tax_enabled,
 COALESCE(json_extract(original,'$.input_credit'),0) AS input_credit,COALESCE(json_extract(original,'$.total'),0) AS invoice_total,
 COALESCE(json_extract(l.value,'$.hsn'),'') AS hsn,COALESCE(json_extract(l.value,'$.treatment'),'NON_GST') AS treatment,
 COALESCE(json_extract(l.value,'$.rate'),0) AS rate,sign*COALESCE(json_extract(l.value,'$.amount'),0) AS taxable_value,
 sign*COALESCE(json_extract(l.value,'$.cgst'),0) AS cgst,sign*COALESCE(json_extract(l.value,'$.sgst'),0) AS sgst,
 sign*COALESCE(json_extract(l.value,'$.igst'),0) AS igst,sign*COALESCE(json_extract(l.value,'$.cess'),0) AS cess
 FROM events e,json_each(e.amounts,'$.lines') l
 WHERE json_extract(original,'$.tax_scheme')='REGULAR'
) `

func gstPreparationQuery(kind string) string {
	if kind == "gstr-1-preparation" {
		return gstPreparationLines + `, classified AS (SELECT *,CASE
   WHEN treatment IN ('NIL','EXEMPT','NON_GST') THEN '8 - Nil/exempt/non-GST'
   WHEN entry='CREDIT_NOTE' AND gstin!='' THEN '9B - CDNR'
   WHEN entry='CREDIT_NOTE' AND (tax_mode IN ('EXPORT_LUT','EXPORT_TAX') OR (gstin='' AND supplier_state!=place_of_supply AND invoice_total>CASE WHEN invoice_date>='2024-08-01' THEN 10000000 ELSE 25000000 END)) THEN '9B - CDNUR'
   WHEN tax_mode IN ('EXPORT_LUT','EXPORT_TAX') THEN '6A - Exports'
   WHEN tax_mode IN ('SEZ_LUT','SEZ_TAX') THEN '6B - SEZ'
   WHEN gstin!='' THEN '4 - B2B'
   WHEN supplier_state!=place_of_supply AND invoice_total>CASE WHEN invoice_date>='2024-08-01' THEN 10000000 ELSE 25000000 END THEN '5 - B2CL'
   ELSE '7 - B2CS' END AS section FROM lines WHERE kind='SALE')
 SELECT section,date,number,invoice,entry,gstin,party,place_of_supply,tax_mode,treatment,hsn,rate,taxable_value,cgst,sgst,igst,cess,
 'PREPARATION: money in paise; review amendments, advances, HSN summary and document series separately' AS review_note
 FROM classified ORDER BY section,date,number,hsn,rate`
	}
	return gstPreparationLines + `, mapped AS (
 SELECT CASE
  WHEN kind='SALE' AND tax_mode='REVERSE_CHARGE' THEN 'REVIEW - Outward reverse charge'
  WHEN kind='SALE' AND tax_mode IN ('EXPORT_LUT','EXPORT_TAX','SEZ_LUT','SEZ_TAX') THEN '3.1(b) - Zero-rated outward'
  WHEN kind='SALE' AND treatment IN ('NIL','EXEMPT') THEN '3.1(c) - Nil/exempt outward'
  WHEN kind='SALE' AND treatment='NON_GST' THEN '3.1(e) - Non-GST outward'
  WHEN kind='SALE' THEN '3.1(a) - Taxable outward'
  WHEN kind='PURCHASE' AND tax_mode='REVERSE_CHARGE' THEN '3.1(d) - Inward reverse charge'
  WHEN kind='PURCHASE' AND tax_enabled=1 AND treatment IN ('NIL','EXEMPT') THEN '5 - Nil/exempt inward'
  WHEN kind='PURCHASE' AND tax_enabled=1 AND treatment='NON_GST' THEN '5 - Non-GST inward'
  WHEN kind='PURCHASE' AND tax_enabled=0 THEN 'REVIEW - Inward classification unavailable'
  ELSE 'REVIEW - Taxable inward source' END AS section,'' AS place_of_supply,taxable_value,cgst,sgst,igst,cess FROM lines
 UNION ALL SELECT CASE WHEN tax_mode='REVERSE_CHARGE' THEN '4(A)(3) - Booked RCM ITC candidate' ELSE '4(A)(5) - Booked other ITC candidate' END,'',0,cgst,sgst,igst,cess FROM lines WHERE kind='PURCHASE' AND input_credit=1 AND tax_enabled=1
 UNION ALL SELECT '3.2 - Interstate unregistered',place_of_supply,taxable_value,0,0,igst,0 FROM lines WHERE kind='SALE' AND gstin='' AND supplier_state!=place_of_supply AND tax_mode='NORMAL' AND treatment='TAXABLE'
 ) SELECT section,place_of_supply,SUM(taxable_value) AS taxable_value,SUM(cgst) AS cgst,SUM(sgst) AS sgst,SUM(igst) AS igst,SUM(cess) AS cess,
 'PREPARATION: money in paise; overlapping sections must not be summed; ITC requires 2B/payment review; no payment, interest or manual-adjustment computation' AS review_note
 FROM mapped GROUP BY section,place_of_supply ORDER BY section,place_of_supply`
}
