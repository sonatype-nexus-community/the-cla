ALTER TABLE signatures
    DROP CONSTRAINT signatures_loginname_key;

ALTER TABLE signatures
    ADD UNIQUE (LoginName, ClaVersion);
