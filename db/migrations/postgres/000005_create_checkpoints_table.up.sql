BEGIN;
CREATE TABLE checkpoints (
  seq         SERIAL          PRIMARY KEY,
  id          UUID            NOT NULL,
  created     BIGINT          NOT NULL,
  updated     BIGINT          NOT NULL,
  listeners   JSON
);
CREATE UNIQUE INDEX checkpoints_id ON checkpoints(id);
COMMIT;