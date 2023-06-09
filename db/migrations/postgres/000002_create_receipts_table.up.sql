BEGIN;
CREATE TABLE receipts (
  seq               SERIAL          PRIMARY KEY,
  id                TEXT            NOT NULL,
  created           BIGINT          NOT NULL,
  updated           BIGINT          NOT NULL,
  block_number      VARCHAR(65),
  tx_index          VARCHAR(65),
  block_hash        TEXT            NOT NULL,
  success           BOOLEAN         NOT NULL,
  protocol_id       TEXT            NOT NULL,
  extra_info        TEXT,
  contract_loc      TEXT
);
CREATE UNIQUE INDEX receipts_id ON receipts(id);
COMMIT;