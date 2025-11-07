-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_transactions_source_account_id ON transactions(source_account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_destination_account_id ON transactions(destination_account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions(created_at);
CREATE INDEX IF NOT EXISTS idx_transactions_idempotency_key ON transactions(idempotency_key);

