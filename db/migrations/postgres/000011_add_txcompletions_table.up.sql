BEGIN;

-- Deliberately very lightweight table, just records when a status change is made to
-- Succeeded or Failed. Write only. Only one record per TX.
CREATE TABLE transaction_completions (
  seq           SERIAL          PRIMARY KEY,
  id            TEXT            NOT NULL,
  status        VARCHAR(65)     NOT NULL,
  time          BIGINT          NOT NULL
);

CREATE UNIQUE INDEX transaction_completions_id ON transaction_completions(id);

COMMIT;