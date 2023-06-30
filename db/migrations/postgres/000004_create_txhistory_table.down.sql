BEGIN;
DROP INDEX IF EXISTS txhistory_id;
DROP INDEX IF EXISTS txhistory_txid;
DROP TABLE txhistory;
COMMIT;
