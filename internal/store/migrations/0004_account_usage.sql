CREATE TABLE account_usage (
 account_id TEXT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
 snapshot TEXT NOT NULL
) STRICT;
