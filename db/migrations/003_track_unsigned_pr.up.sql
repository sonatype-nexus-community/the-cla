CREATE TABLE unsigned_pr
(
    Id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_owner varchar(250) NOT NULL,
    repo_name  varchar(250) NOT NULL,
    pr_number  int          NOT NULL,
    login_name varchar(250) NOT NULL,
    ClaVersion varchar(10),
    CheckedAt  timestamp    NOT NULL,
    UNIQUE (repo_name, pr_number, login_name, ClaVersion)
);
