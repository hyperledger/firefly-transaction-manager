BEGIN;

CREATE TABLE checkpoints (
  seq          SERIAL        PRIMARY KEY,
  stream_id    UUID          NOT NULL,
  listeners    JSONB         NOT NULL 
  time         BIGINT        NOT NULL,
);

CREATE UNIQUE INDEX checkpoints_streamId on checkpoints(stream_id);
COMMIT;