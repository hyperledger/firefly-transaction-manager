BEGIN;
CREATE TABLE listeners (
  seq                    SERIAL          PRIMARY KEY,
  id                     UUID            NOT NULL,
  created                BIGINT          NOT NULL,
  updated                BIGINT          NOT NULL,
  name                   TEXT,
  stream_id              UUID            NOT NULL,
  filters                TEXT,
  options                TEXT,
  signature              TEXT,
  from_block             TEXT
);
CREATE UNIQUE INDEX listeners_id ON listeners(id);
CREATE UNIQUE INDEX listeners_name ON listeners(name); -- global uniqueness on names
CREATE INDEX listeners_stream ON listeners(stream_id);
COMMIT;