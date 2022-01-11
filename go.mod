module github.com/dhis2-sre/im-manager

go 1.16

require (
	github.com/dhis2-sre/im-user v0.4.0
	github.com/dhis2-sre/instance-queue v0.2.0
	github.com/fatih/color v1.9.0 // indirect
	github.com/gin-contrib/cors v1.3.1
	github.com/gin-gonic/gin v1.7.7
	github.com/go-openapi/runtime v0.21.0
	github.com/gofrs/uuid v4.2.0+incompatible
	github.com/google/wire v0.5.0
	github.com/hashicorp/go-multierror v1.1.0 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/lestrrat-go/jwx v1.2.14
	github.com/rabbitmq/amqp091-go v1.2.0
	github.com/stretchr/testify v1.7.0
	go.mozilla.org/sops/v3 v3.7.1
	google.golang.org/api v0.56.0 // indirect
	gopkg.in/ini.v1 v1.63.2 // indirect
	gorm.io/driver/postgres v1.2.3
	gorm.io/gorm v1.22.4
	k8s.io/api v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/client-go v0.22.4
)
