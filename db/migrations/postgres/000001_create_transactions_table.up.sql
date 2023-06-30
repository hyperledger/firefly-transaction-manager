BEGIN;
CREATE TABLE transactions (
  seq           SERIAL          PRIMARY KEY,
  id            TEXT            NOT NULL,
  created       BIGINT          NOT NULL,
  updated       BIGINT          NOT NULL,
  status        VARCHAR(65)     NOT NULL,
  delete        BIGINT,
  tx_from       TEXT,
  tx_to         TEXT,
  tx_nonce      VARCHAR(65),
  tx_gas        VARCHAR(65),
  tx_value      VARCHAR(65),
  tx_gasprice   TEXT,
  tx_data       TEXT            NOT NULL,
  tx_hash       TEXT            NOT NULL,
  policy_info   TEXT,
  first_submit  BIGINT,
  last_submit   BIGINT,
  error_message TEXT            NOT NULL
);
CREATE UNIQUE INDEX transactions_id ON transactions(id);
CREATE UNIQUE INDEX transactions_nonce ON transactions(tx_from, tx_nonce);
CREATE INDEX transactions_hash ON transactions(tx_hash);
COMMIT;