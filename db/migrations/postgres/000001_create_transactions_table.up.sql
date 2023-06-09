BEGIN;
CREATE TABLE transactions (
  seq           SERIAL          PRIMARY KEY,
  id            TEXT            NOT NULL,
  created       BIGINT          NOT NULL,
  updated       BIGINT          NOT NULL,
  status        VARCHAR(65)     NOT NULL,
  delete        BIGINT,
  tx_from       TEXT            NOT NULL,
  tx_to         TEXT            NOT NULL,
  tx_nonce      VARCHAR(65)     NOT NULL,
  tx_gas        VARCHAR(65)     NOT NULL,
  tx_value      VARCHAR(65)     NOT NULL,
  tx_gasprice   TEXT,
  tx_data       TEXT            NOT NULL,
  tx_hash       TEXT            NOT NULL,
  policy_info   TEXT,
  first_submit  BIGINT,
  last_submit   BIGINT,
  error_message TEXT            NOT NULL
);
CREATE UNIQUE INDEX transactions_id ON transactions(id);
CREATE INDEX transactions_from ON transactions(tx_from);
CREATE INDEX transactions_nonce ON transactions(tx_nonce);
CREATE INDEX transactions_hash ON transactions(tx_hash);
COMMIT;