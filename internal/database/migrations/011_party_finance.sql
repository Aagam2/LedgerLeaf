CREATE TABLE party_openings (
 id INTEGER PRIMARY KEY,business_id INTEGER NOT NULL,party_id INTEGER NOT NULL,date TEXT NOT NULL,amount INTEGER NOT NULL CHECK(amount>0),reference TEXT NOT NULL,
 UNIQUE(business_id,party_id),UNIQUE(business_id,reference),FOREIGN KEY(business_id,party_id) REFERENCES parties(business_id,id)
);
CREATE TABLE party_advances (
 id INTEGER PRIMARY KEY,business_id INTEGER NOT NULL,party_id INTEGER NOT NULL,date TEXT NOT NULL,amount INTEGER NOT NULL CHECK(amount>0),mode TEXT NOT NULL CHECK(mode IN ('CASH','BANK')),reference TEXT NOT NULL,
 UNIQUE(business_id,id),UNIQUE(business_id,reference),FOREIGN KEY(business_id,party_id) REFERENCES parties(business_id,id)
);
CREATE TABLE advance_usage (
 id INTEGER PRIMARY KEY,business_id INTEGER NOT NULL,advance_id INTEGER NOT NULL,document_id INTEGER,date TEXT NOT NULL,amount INTEGER NOT NULL CHECK(amount>0),kind TEXT NOT NULL CHECK(kind IN ('ALLOCATE','REFUND')),reference TEXT NOT NULL,
 UNIQUE(business_id,reference),FOREIGN KEY(business_id,advance_id) REFERENCES party_advances(business_id,id),FOREIGN KEY(business_id,document_id) REFERENCES documents(business_id,id)
);
CREATE TABLE opening_settlements (
 id INTEGER PRIMARY KEY,business_id INTEGER NOT NULL,party_id INTEGER NOT NULL,date TEXT NOT NULL,amount INTEGER NOT NULL CHECK(amount>0),reference TEXT NOT NULL,
 UNIQUE(business_id,reference),FOREIGN KEY(business_id,party_id) REFERENCES parties(business_id,id)
);
CREATE TRIGGER party_openings_no_update BEFORE UPDATE ON party_openings BEGIN SELECT RAISE(ABORT,'opening balances are immutable'); END;
CREATE TRIGGER party_openings_no_delete BEFORE DELETE ON party_openings BEGIN SELECT RAISE(ABORT,'opening balances are immutable'); END;
CREATE TRIGGER advances_no_update BEFORE UPDATE ON party_advances BEGIN SELECT RAISE(ABORT,'advances are immutable'); END;
CREATE TRIGGER advances_no_delete BEFORE DELETE ON party_advances BEGIN SELECT RAISE(ABORT,'advances are immutable'); END;
CREATE TRIGGER advance_usage_no_update BEFORE UPDATE ON advance_usage BEGIN SELECT RAISE(ABORT,'advance usage is immutable'); END;
CREATE TRIGGER advance_usage_no_delete BEFORE DELETE ON advance_usage BEGIN SELECT RAISE(ABORT,'advance usage is immutable'); END;
CREATE TRIGGER opening_settlements_no_update BEFORE UPDATE ON opening_settlements BEGIN SELECT RAISE(ABORT,'opening settlements are immutable'); END;
CREATE TRIGGER opening_settlements_no_delete BEFORE DELETE ON opening_settlements BEGIN SELECT RAISE(ABORT,'opening settlements are immutable'); END;
