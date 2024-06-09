BEGIN;
CREATE INDEX transactions_status ON transactions(status);
COMMIT;