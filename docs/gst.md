# GST calculation and preparation

Implemented: GSTIN structural/checksum and PAN matching validation, regular/composition/unregistered businesses, party GSTIN/state, immutable effective-dated HSN/SAC and configured rates. Rates use basis points; money uses paise. Tax-inclusive values are decomposed by the sum of applicable percentage rates, with deterministic residual-paise allocation so the gross remains exact. Intra-state tax splits the GST amount into CGST and SGST/UTGST; the latter is labelled generically, with a one-paise residual assigned to CGST when necessary.

Users explicitly choose place of supply, inclusive/exclusive prices and supply treatment: NORMAL, EXPORT_LUT, EXPORT_TAX, SEZ_LUT, SEZ_TAX or REVERSE_CHARGE. Export/SEZ modes force interstate treatment; LUT modes collect zero tax. Reverse charge excludes assessed tax from the supplier amount: purchase posting records the recipient tax liability and eligible input credit or inventory cost. Input-credit recognition is an explicit user choice, not an eligibility decision by the app. Composition sales collect no GST and produce a bill of supply. Regular sales with only non-taxable lines also produce a bill of supply.

Finalized snapshots preserve registration, HSN, treatment, rates, component amounts and place of supply. Changed registration/rate data requires draft review before finalization. Sale revenue excludes collected tax; purchase tax enters input-tax assets when eligible or inventory cost otherwise. Partial returns allocate original tax/cost cumulatively so returning all quantities clears every rounding remainder. GST preparation report rows include invoices and dated credit notes, and export as CSV/XLSX/PDF.

Invoice-level discounts and allocated charges change the line consideration before configured taxes are calculated. Allocation follows the discounted item values; inclusive-price invoices also treat those adjustments as tax-inclusive. This implementation assumes the adjustments share the allocated items’ tax treatment and does not decide whether that assumption is legally appropriate. Payable rupee rounding uses a separate accounting entry and leaves tax components unchanged.

## Scope requiring review

This is configured arithmetic and bookkeeping, not a legal classification or filing service. Current product rates are not seeded or inferred. The user must confirm place-of-supply rules, export/SEZ authorisation and LUT conditions, reverse-charge applicability, input-credit timing/eligibility and tax registration status. Configured quantity-based cess is implemented. Specialized valuation/margin schemes, GST on advances, imports/customs, complete portal return schemas and automatic HSN/turnover classification remain pending. No GST portal/e-invoice/e-way-bill integration is included.

Both invoice-accompanying delivery challans and independent inventory dispatches are implemented, with partial and consolidated invoicing. PDF physical printing and signatures have not been validated as a legal issuance workflow.

## Primary references consulted

- [CBIC Rule 46: tax invoice particulars](https://taxinformation.cbic.gov.in/content-page/explore-rules/1000136/1000001)
- [CBIC CGST Rules, Rule 35: tax-inclusive value](https://cbic-gst.gov.in/pdf/01062021-CGST-Rules-2017-Part-A-Rules.pdf)
- [CBIC tax invoice, credit/debit notes rules](https://cbic-gst.gov.in/gst-invoice-rules.html)

References informed implementation; they are not a substitute for checking amendments and the business-specific legal treatment.

## GSTR-1 and GSTR-3B preparation worksheets

The user selected preparation exports, not portal upload files. Reports includes `gstr-1-preparation` and `gstr-3b-preparation`, with CSV/XLSX/PDF output and date filtering. Data uses frozen original invoice registration/supply fields and credit-note amounts on the credit-note date. Money columns in downloads are integer paise.

GSTR-1 worksheets label recorded B2B/B2CL/B2CS, export/SEZ, nil/exempt/non-GST and applicable credit-note lines for review. The B2CL threshold uses the original invoice period (₹1 lakh from August 2024, ₹2.5 lakh earlier). GSTR-3B worksheets show recorded outward/inward RCM categories, interstate unregistered details and booked ITC candidates. Candidate credit requires eligibility, payment and GSTR-2B reconciliation. Unknown inward classification is explicit. Sections overlap and must not be summed together.

These worksheets do not infer missing customs/ISD/e-commerce/advance/amendment records or calculate filing settlement, interest or every manual reversal. Use the existing HSN summary and source documents alongside the preparation worksheets. Compliance verification is deferred.

Implementation references: [GST Portal GSTR-1 guide](https://tutorial.gst.gov.in/userguide/returns/Creation_of_Outward_Supplies_Return_in_GSTR-1.htm), [GST Portal liability/ITC comparison guide](https://tutorial.gst.gov.in/userguide/inputtaxcredit/Manual_comparison_of_liability_declared_and_ITC_claimed.htm), and [GST Portal GSTR-2B FAQ](https://tutorial.gst.gov.in/userguide/returns/FAQ_gstr2b.htm).
