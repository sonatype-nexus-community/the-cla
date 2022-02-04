CREATE TABLE unsigned_pr
(
    Id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    RepoOwner varchar(250) NOT NULL,
    RepoName  varchar(250) NOT NULL,
    sha       varchar(250) NOT NULL,
    PRNumber  int          NOT NULL,
    AppID     int          NOT NULL,
    InstallID int          NOT NULL,
    UNIQUE (RepoName, PRNumber)
);

CREATE TABLE unsigned_user
(
    Id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    UnsignedPRID UUID         NOT NULL,
    LoginName    varchar(250) NOT NULL,
    Email        varchar(250),
    GivenName    varchar(250),
    ClaVersion   varchar(10)  NOT NULL,
    CheckedAt    timestamp    NOT NULL,
    UNIQUE (UnsignedPRID, LoginName, ClaVersion),
    FOREIGN KEY (UnsignedPRID) REFERENCES unsigned_pr (Id)
);
