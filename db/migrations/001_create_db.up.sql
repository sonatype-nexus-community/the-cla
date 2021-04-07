
BEGIN;

CREATE SCHEMA the_cla;

CREATE TABLE the_cla.signatures(
    Id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    LoginName varchar(250) NOT NULL UNIQUE,
    Email varchar(250),
    GivenName varchar(250),
    SignedAt timestamp NOT NULL,
    ClaVersion varchar(10)
);

COMMIT;
