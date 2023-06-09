BEGIN;
DROP INDEX transactions_id;
DROP INDEX transactions_from;
DROP INDEX transactions_nonce;
DROP INDEX transactions_hash;
DROP TABLE transactions;
COMMIT;
