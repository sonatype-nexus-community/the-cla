CREATE TABLE unsigned_pr
(
    Id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_owner varchar(250) NOT NULL,
    repo_name  varchar(250) NOT NULL,
    sha        varchar(250) NOT NULL,
    pr_number  int          NOT NULL,
    app_id     int          NOT NULL,
    install_id int          NOT NULL,
    login_name varchar(250) NOT NULL,
    given_name varchar(250),
    email      varchar(250),
    ClaVersion varchar(10)  NOT NULL,
    CheckedAt  timestamp    NOT NULL,
    UNIQUE (repo_name, pr_number, login_name, ClaVersion)
);
