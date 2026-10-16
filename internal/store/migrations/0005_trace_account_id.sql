-- No FK: historical account attribution survives removing a vault account.
ALTER TABLE traces ADD COLUMN account_id TEXT;
CREATE INDEX traces_account_started_idx ON traces(account_id, started_at);
