module github.com/sonatype-nexus-community/the-cla

go 1.16

require (
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/bradleyfalzon/ghinstallation/v2 v2.0.4
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/golang-migrate/migrate/v4 v4.15.1
	github.com/google/go-github/v42 v42.0.0
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/joho/godotenv v1.4.0
	github.com/labstack/echo/v4 v4.6.3
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/stretchr/testify v1.7.0
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	gopkg.in/go-playground/webhooks.v5 v5.17.0
)

// fix: CVE-2021-21334 in github.com/containerd/containerd v1.4.3
// fix: CVE-2021-32760 in github.com/containerd/containerd v1.4.4
//replace github.com/containerd/containerd => github.com/containerd/containerd v1.4.8

// fix: CVE-2021-20329 in go.mongodb.org/mongo-driver v1.1.0
//replace go.mongodb.org/mongo-driver => go.mongodb.org/mongo-driver v1.5.1

// fix: CVE-2021-3121 in pkg:golang/github.com/gogo/protobuf@1.3.1
replace github.com/dhui/dktest => github.com/dhui/dktest v0.3.4

// fix: SONATYPE-2019-0702 in github.com/gobuffalo/packr/v2 v2.2.0
//replace github.com/gobuffalo/packr/v2 => github.com/gobuffalo/packr/v2 v2.3.2

// fix: CVE-2020-15114 in etcd v3.3.10
//replace github.com/coreos/etcd => github.com/coreos/etcd v3.3.24+incompatible

// fix: sonatype-2021-0853 in github.com/jackc/pgproto3 v1.1.0
//replace github.com/jackc/pgproto3 => github.com/jackc/pgproto3/v2 v2.1.1

// fix: CVE-2020-26160 in github.com/dgrijalva/jwt-go v3.2.0
//replace github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt/v4 v4.1.0

// fix: CVE-2020-8911 in github.com/aws/aws-sdk-go v1.17.7
replace github.com/aws/aws-sdk-go => github.com/aws/aws-sdk-go v1.33.21

// fix: CVE-2021-41190 in github.com/opencontainers/image-spec v1.0.1
replace github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2
