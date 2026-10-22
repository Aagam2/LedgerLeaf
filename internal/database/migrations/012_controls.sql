CREATE TABLE business_controls(business_id INTEGER PRIMARY KEY REFERENCES businesses(id),closed_through TEXT NOT NULL DEFAULT '');
CREATE TRIGGER closed_journal BEFORE INSERT ON journals WHEN NEW.date<=(SELECT closed_through FROM business_controls WHERE business_id=NEW.business_id) BEGIN SELECT RAISE(ABORT,'financial period is locked'); END;
CREATE TRIGGER closed_stock BEFORE INSERT ON stock_movements WHEN NEW.date<=(SELECT closed_through FROM business_controls WHERE business_id=NEW.business_id) BEGIN SELECT RAISE(ABORT,'financial period is locked'); END;
CREATE TRIGGER closed_payments BEFORE INSERT ON payments WHEN NEW.date<=(SELECT closed_through FROM business_controls WHERE business_id=NEW.business_id) BEGIN SELECT RAISE(ABORT,'financial period is locked'); END;
CREATE TRIGGER closed_invoice BEFORE UPDATE ON documents WHEN NEW.status='FINALIZED' AND NEW.date<=(SELECT closed_through FROM business_controls WHERE business_id=NEW.business_id) BEGIN SELECT RAISE(ABORT,'financial period is locked'); END;
CREATE TABLE bank_accounts(business_id INTEGER NOT NULL,code TEXT NOT NULL,name TEXT NOT NULL,account_number TEXT NOT NULL DEFAULT '',ifsc TEXT NOT NULL DEFAULT '',PRIMARY KEY(business_id,code),FOREIGN KEY(business_id,code) REFERENCES accounts(business_id,code));
INSERT INTO bank_accounts(business_id,code,name) SELECT business_id,code,'Bank' FROM accounts WHERE code='1010';
CREATE TABLE bank_reconciliations(id INTEGER PRIMARY KEY,business_id INTEGER NOT NULL,journal_line_id INTEGER NOT NULL,statement_date TEXT NOT NULL,statement_reference TEXT NOT NULL,UNIQUE(business_id,journal_line_id),FOREIGN KEY(journal_line_id) REFERENCES journal_lines(id),FOREIGN KEY(business_id) REFERENCES businesses(id));
ALTER TABLE payments ADD COLUMN account_code TEXT NOT NULL DEFAULT '';
