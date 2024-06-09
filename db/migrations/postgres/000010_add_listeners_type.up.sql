BEGIN;

ALTER TABLE listeners ADD COLUMN "type" VARCHAR(64);
UPDATE listeners SET "type" = 'events';
ALTER TABLE listeners ALTER COLUMN "type" SET NOT NULL;

COMMIT;