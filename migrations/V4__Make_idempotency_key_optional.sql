-- Make idempotency_key optional (nullable) and add index for performance
ALTER TABLE transactions ALTER COLUMN idempotency_key DROP NOT NULL;
DROP INDEX IF EXISTS idx_transactions_idempotency_key;
CREATE UNIQUE INDEX idx_transactions_idempotency_key ON transactions (idempotency_key) WHERE idempotency_key IS NOT NULL;