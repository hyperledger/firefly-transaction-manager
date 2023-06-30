BEGIN;
DROP INDEX confirmations_id;
DROP INDEX confirmations_txid;
DROP TABLE confirmations;
COMMIT;
