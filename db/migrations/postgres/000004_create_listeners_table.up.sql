BEGIN;

CREATE TABLE listeners (
  seq                      SERIAL          PRIMARY KEY,
  id                       UUID            NOT NULL,
  name                     VARCHAR(64),
  stream_id                UUID          NOT NULL,
  eth_compat_address       VARCHAR(64),
  eth_compat_event         JSONB,
  eth_compat_methods       JSONB,
  filters                  JSONB,
  options                  JSONB,
  signature                TEXT,
  from_block               VARCHAR(64)
  created                  BIGINT          NOT NULL,
  updated                  BIGINT,
);

CREATE UNIQUE INDEX listener_id on listeners(id);
COMMIT;