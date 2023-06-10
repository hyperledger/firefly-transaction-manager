BEGIN;
DROP INDEX listeners_id;
DROP INDEX listeners_name;
DROP INDEX listeners_stream;
DROP TABLE listeners;
COMMIT;
