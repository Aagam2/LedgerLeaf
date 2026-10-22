-- Snapshot semantics now include invoice-level allocations and payable rounding.
-- Bump the schema even though those fields are JSON: older builds must not open
-- these records and reverse them without the corresponding rounding journal.
INSERT OR IGNORE INTO accounts(business_id,code,name,kind)
SELECT id,'5300','Invoice rounding','EXPENSE' FROM businesses;
