BEGIN;
CREATE TABLE confirmations (
  seq               SERIAL          PRIMARY KEY,
  id                UUID            NOT NULL,
  created           BIGINT          NOT NULL,
  updated           BIGINT          NOT NULL,
  tx_id             TEXT            NOT NULL,
  block_number      BIGINT          NOT NULL,
  block_hash        TEXT            NOT NULL,
  parent_hash       TEXT            NOT NULL
);
CREATE UNIQUE INDEX confirmations_id ON confirmations(id);
CREATE INDEX confirmations_txid ON confirmations(tx_id);
COMMIT;