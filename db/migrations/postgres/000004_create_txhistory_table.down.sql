BEGIN;
DROP INDEX txhistory_id;
DROP INDEX txhistory_txid;
DROP TABLE txhistory;
COMMIT;
