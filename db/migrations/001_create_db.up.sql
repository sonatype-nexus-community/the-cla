
BEGIN;

CREATE EXTENSION pgcrypto;

CREATE TABLE signatures(
    Id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    LoginName varchar(250) NOT NULL UNIQUE,
    Email varchar(250),
    GivenName varchar(250),
    SignedAt timestamp NOT NULL,
    ClaVersion varchar(10)
);

COMMIT;
