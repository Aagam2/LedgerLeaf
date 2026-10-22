ALTER TABLE users ADD COLUMN restricted INTEGER NOT NULL DEFAULT 0 CHECK(restricted IN (0,1));
CREATE TABLE user_businesses (
 user_id INTEGER NOT NULL REFERENCES users(id),
 business_id INTEGER NOT NULL REFERENCES businesses(id),
 PRIMARY KEY(user_id,business_id)
);
