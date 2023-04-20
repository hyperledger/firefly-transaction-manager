BEGIN;

CREATE TABLE streams (
  seq                      SERIAL          PRIMARY KEY,
  id                       UUID            NOT NULL,
  name                     VARCHAR(64),
  suspended                BOOLEAN,
  type                     VARCHAR(64),     
  error_handling           VARCHAR(64)     NOT NULL,
  batch_size               BIGINT          NOT NULL,           
  batch_timeout            BIGINT          NOT NULL,           
  retry_timeout            BIGINT          NOT NULL,           
  blocked_retry_delay      BIGINT          NOT NULL,           
  eth_batch_timeout        BIGINT, 
  eth_retry_timeout        BIGINT,
  eth_blocked_retry_delay  BIGINT,
  webhook                  JSONB,
  websocket                JSONB,
  created                  BIGINT          NOT NULL,
  updated                  BIGINT,
);

CREATE UNIQUE INDEX streams_id on streams(id);
COMMIT;