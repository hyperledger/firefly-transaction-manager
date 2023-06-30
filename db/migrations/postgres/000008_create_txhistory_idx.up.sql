BEGIN;
-- Allow nil data on transactions, for example for a simple transfer operation
ALTER TABLE transactions ALTER COLUMN tx_data DROP NOT NULL;

-- Correct TXHistory indexes if created by an invalid 000004 migration (no longer in codebase).
DROP INDEX IF EXISTS txhistory_id;
DROP INDEX IF EXISTS txhistory_txid;

-- Create corrected TXHistory indexes
CREATE UNIQUE INDEX txhistory_id ON txhistory(id);
CREATE INDEX txhistory_txid ON txhistory(tx_id);
COMMIT;