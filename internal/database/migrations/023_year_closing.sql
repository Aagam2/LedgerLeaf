CREATE TABLE year_closings(business_id INTEGER NOT NULL REFERENCES businesses(id),date TEXT NOT NULL,reference TEXT NOT NULL,net_profit INTEGER NOT NULL,PRIMARY KEY(business_id,date),UNIQUE(business_id,reference));
CREATE TRIGGER year_closings_no_update BEFORE UPDATE ON year_closings BEGIN SELECT RAISE(ABORT,'year closings are immutable'); END;
CREATE TRIGGER year_closings_no_delete BEFORE DELETE ON year_closings BEGIN SELECT RAISE(ABORT,'year closings are immutable'); END;
INSERT OR IGNORE INTO accounts(business_id,code,name,kind) SELECT id,'3100','Retained earnings','EQUITY' FROM businesses;
