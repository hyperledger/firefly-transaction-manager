BEGIN;
DROP INDEX transactions_id;
DROP INDEX transactions_nonce;
DROP TABLE transactions;
COMMIT;
