BEGIN;

ALTER TABLE signatures
    DROP COLUMN ClaTextUrl,
    DROP COLUMN ClaText;

COMMIT;
