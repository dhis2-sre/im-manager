module github.com/dhis2-sre/im-manager

go 1.16

require (
	github.com/dhis2-sre/im-users v0.0.0-20211209075020-eee17751df4f
	github.com/gin-contrib/cors v1.3.1
	github.com/gin-gonic/gin v1.7.7
	github.com/go-playground/validator/v10 v10.9.0 // indirect
	github.com/gofrs/uuid v4.2.0+incompatible
	github.com/google/wire v0.5.0
	github.com/jackc/pgx/v4 v4.14.1 // indirect
	github.com/jinzhu/now v1.1.4 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/lestrrat-go/jwx v1.2.11
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/stretchr/testify v1.7.0
	go.mozilla.org/sops/v3 v3.7.1
	golang.org/x/crypto v0.0.0-20211202192323-5770296d904e // indirect
	golang.org/x/sys v0.0.0-20211205182925-97ca703d548d // indirect
	gorm.io/driver/postgres v1.2.3
	gorm.io/gorm v1.22.4
	k8s.io/client-go v0.22.4
)
