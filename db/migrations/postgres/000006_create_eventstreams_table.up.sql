BEGIN;
CREATE TABLE eventstreams (
  seq                    SERIAL          PRIMARY KEY,
  id                     UUID            NOT NULL,
  created                BIGINT          NOT NULL,
  updated                BIGINT          NOT NULL,
  name                   TEXT,
  suspended              BOOLEAN,
  stream_type            TEXT,
  error_handling         TEXT,
  batch_size             BIGINT,
  batch_timeout          TEXT            NOT NULL,
  retry_timeout          TEXT            NOT NULL,
  blocked_retry_timeout  TEXT            NOT NULL,
  webhook_config         TEXT,
  websocket_config       TEXT
);
CREATE UNIQUE INDEX eventstreams_id ON eventstreams(id);
CREATE UNIQUE INDEX eventstreams_name ON eventstreams(name);
COMMIT;