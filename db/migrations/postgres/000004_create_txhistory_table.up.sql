BEGIN;
CREATE TABLE txhistory (
  seq               SERIAL          PRIMARY KEY,
  id                UUID            NOT NULL,
  time              BIGINT          NOT NULL,
  last_occurrence   BIGINT          NOT NULL,
  tx_id             TEXT            NOT NULL,
  status            TEXT            NOT NULL,
  action            TEXT            NOT NULL,
  count             INT             NOT NULL,
  error             TEXT,
  error_time        BIGINT,
  info              TEXT
);
CREATE UNIQUE INDEX txhistory_id ON confirmations(id);
CREATE INDEX txhistory_txid ON confirmations(tx_id);
COMMIT;