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
COMMIT;