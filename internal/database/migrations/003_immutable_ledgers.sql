CREATE TRIGGER stock_no_update BEFORE UPDATE ON stock_movements BEGIN SELECT RAISE(ABORT,'stock movements are immutable'); END;
CREATE TRIGGER stock_no_delete BEFORE DELETE ON stock_movements BEGIN SELECT RAISE(ABORT,'stock movements are immutable'); END;
CREATE TRIGGER journals_no_update BEFORE UPDATE ON journals BEGIN SELECT RAISE(ABORT,'journals are immutable'); END;
CREATE TRIGGER journals_no_delete BEFORE DELETE ON journals BEGIN SELECT RAISE(ABORT,'journals are immutable'); END;
CREATE TRIGGER lines_no_update BEFORE UPDATE ON journal_lines BEGIN SELECT RAISE(ABORT,'journal lines are immutable'); END;
CREATE TRIGGER lines_no_delete BEFORE DELETE ON journal_lines BEGIN SELECT RAISE(ABORT,'journal lines are immutable'); END;
CREATE TRIGGER audit_no_update BEFORE UPDATE ON audit_log BEGIN SELECT RAISE(ABORT,'audit log is immutable'); END;
CREATE TRIGGER audit_no_delete BEFORE DELETE ON audit_log BEGIN SELECT RAISE(ABORT,'audit log is immutable'); END;
