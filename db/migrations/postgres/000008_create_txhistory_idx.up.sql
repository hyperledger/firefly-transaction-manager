BEGIN;
CREATE UNIQUE INDEX txhistory_id ON txhistory(id);
CREATE INDEX txhistory_txid ON txhistory(tx_id);
COMMIT;