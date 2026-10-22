# Accounting implemented so far

All postings use integer paise and must balance before insertion. They are committed atomically with the originating stock/document/payment operation. No approximate financial totals are used.

| Transaction | Debit | Credit |
|---|---|---|
| Opening stock | Inventory | Opening equity |
| Purchase finalization | Inventory | Accounts payable |
| Sale finalization | Accounts receivable | Sales revenue |
| Sale stock cost | Cost of goods sold | Inventory |
| Customer payment | Cash or bank | Accounts receivable |
| Supplier payment | Accounts payable | Cash or bank |

Inventory uses moving weighted-average cost within each product/warehouse. For a sale, allocated cost is the rounded ratio of remaining inventory value to remaining quantity, multiplied by quantity sold. Selling all remaining quantity consumes all remaining value. Equal-date transactions use posting order. Backdated stock postings are rejected. Service sales do not move inventory or post cost of goods sold; service purchases are unavailable.

Manual CASH payments use the cash account; UPI, BANK_TRANSFER, CHEQUE, CARD and OTHER use the bank account. No funds are moved electronically. There is currently one cash and one bank control account per business, not configurable multiple accounts. A payment allocates directly to one finalized document; amounts greater than outstanding are rejected. Repeating the same request is safe; using the key with changed content fails.

Full sales returns reverse receivable/revenue and restore original quantity/cost, reversing COGS. Purchase returns remove stock at current weighted-average value, with any original-cost difference in stock adjustment account 5100. Paid amounts are recorded as refunded via cash/bank. Partial returns are not implemented.

Stock additions debit inventory and credit stock adjustment (5100); removals/damage reverse those accounts. Transfers preserve total inventory value. Paid expenses debit 5200 and credit cash/bank. Manual journals require balanced positive lines and allow cash/bank/equity/expense plus custom accounts; subledger control accounts are excluded. Manual and expense reversals append opposite lines once.

Date-filtered reports derive from journal lines. Profit/loss uses the selected interval; balance sheet includes all postings through its cutoff and accumulated earnings. CSV exports contain integer paise and thousandths for exact interchange. Party opening balances, ageing, financial-year closing, recurring expenses, advances and reconciliation remain pending.
