CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

BEGIN;
CREATE TABLE transactions (
  seq                      SERIAL          PRIMARY KEY,
  id                       VARCHAR(64)     PRIMARY KEY,
  status                   VARCHAR(64)     NOT NULL,
  nonce                    BIGINT          NOT NULL,
  gas                      BIGINT          NOT NULL,
  transaction_headers      JSONB           NOT NULL,
  transaction_data         TEXT            NOT NULL,
  transaction_hash         VARCHAR(64),
  gas_price                JSONB, 
  policy_info              JSONB, 
  first_submit             BIGINT,
  last_submit              BIGINT,
  receipt                  JSONB,
  confirmations            JSONB,
  error_msg                TEXT,
  history                  JSONB,
  history_summary          JSONB,
  created                  BIGINT     NOT NULL,
  updated                  BIGINT,
  delete_requested         BIGINT,
);

CREATE UNIQUE INDEX transactions_id on transactions(id);
CREATE UNIQUE INDEX transactions_nonce_per_signer ON transactions( (transaction_headers->>'from'), nonce ) ;
COMMIT;