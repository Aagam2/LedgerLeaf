# Accounting implemented so far

All postings use integer paise and must balance before insertion. They are committed atomically with the originating stock/document/payment operation. No approximate financial totals are used. This describes implemented behavior; specialized tax and correction workflows remain incomplete.

| Transaction | Debit | Credit |
| --- | --- | --- |
| Opening stock | Inventory (1200) | Opening equity (3000) |
| Ordinary purchase finalization | Inventory and eligible input tax | Accounts payable (2000) and applicable reverse-charge liabilities |
| Sale finalization | Accounts receivable (1100) | Sales revenue (4000) and applicable output tax |
| Sale stock cost | Cost of goods sold (5000) | Inventory (1200) |
| Customer payment | Cash or selected bank | Accounts receivable (1100) |
| Supplier payment | Accounts payable (2000) | Cash or selected bank |
| Goods receipt before invoice | Inventory (1200) | Goods received, invoice pending (2050) |
| Dispatch before invoice | Goods dispatched, invoice pending (1250) | Inventory (1200) |
| Paid expense | Expenses (5200) | Cash or selected bank |

Inventory uses moving weighted-average cost within each product/warehouse. For a sale, allocated cost is the rounded ratio of remaining inventory value to remaining quantity, multiplied by quantity sold. Selling all remaining quantity consumes all remaining value. Equal-date transactions use posting order. Backdated stock postings are rejected. Service sales do not move inventory or post cost of goods sold; service purchases are unavailable. Alternate-unit quantities convert to base-unit thousandths before stock posting.

CASH payments use cash account 1000. Other supported payment modes use the default bank 1010 or a configured bank account in the same business. Additional banks have their own asset ledger accounts and manual statement reconciliation. Ordinary and recurring expenses also accept these named banks. Payments allocate to finalized invoices; overpayment is rejected. Idempotency keys prevent duplicate postings and reject changed content under the same key.

Full returns/cancellations and partial credit notes append corrections while preserving original invoice snapshots. Sale returns restore original quantity and cost; purchase returns remove current weighted-average stock value and post any cost difference to stock adjustment account 5100. Refund limits and cumulative rounding prevent over-refunding or losing residual paise. Invoice-level discounts/charges allocate across lines; payable rounding uses account 5300.

Goods-receipt invoice matching clears the uninvoiced liability without receiving stock twice. Partial and consolidated matching retain immutable quantities/cost allocations. Dispatch invoicing recognizes revenue/tax and transfers the allocated dispatch value to cost of goods sold without issuing warehouse stock again. Uninvoiced dispatch quantities can return to the warehouse; invoiced quantities use invoice returns.

Stock additions debit inventory and credit stock adjustment (5100); removals/damage reverse those accounts. Transfers preserve total inventory value. Manual journals require balanced positive lines and exclude protected inventory/receivable/payable control accounts. Manual and expense reversals append opposite lines once. Expense categories and attachments are stored with the posting; recurring schedules require confirmation of each paid occurrence.

Party opening balances, opening-balance settlement, advances, invoice allocation and unused-advance refunds use dedicated party-finance workflows. Recording an advance does not implement specialized GST-on-advance handling.

Date-filtered reports derive from journal lines. Profit/loss uses the selected interval and excludes year-closing journals; the balance sheet includes postings through its cutoff and accumulated earnings. Financial-year closing previews and then recalculates income/expense balances, transfers them to retained earnings (3100), and permanently locks postings through the closing date. Administrators can set ordinary period locks but cannot reopen a finalized year closing. CSV monetary values use integer paise; quantities use integer thousandths for exact interchange.
