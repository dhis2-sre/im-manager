
<a name="v0.59.2"></a>
## [v0.59.2](https://github.com/dhis2-sre/im-manager/compare/v0.59.1...v0.59.2)

> 2025-10-09

### Chore

* add ALLOW_SUSPEND to request body ([#1254](https://github.com/dhis2-sre/im-manager/issues/1254))
* generate change log for v0.59.1

### Docs

* update command order in release instructions ([#1249](https://github.com/dhis2-sre/im-manager/issues/1249))


<a name="v0.59.1"></a>
## [v0.59.1](https://github.com/dhis2-sre/im-manager/compare/v0.59.0...v0.59.1)

> 2025-10-08

### Chore

* add ALLOW_SUSPEND param to deploy script ([#1250](https://github.com/dhis2-sre/im-manager/issues/1250))
* generate change log for v0.59.0


<a name="v0.59.0"></a>
## [v0.59.0](https://github.com/dhis2-sre/im-manager/compare/v0.58.0...v0.59.0)

> 2025-10-06

### Chore

* use empty string for the hostname to prevent cross-subdomain sharing ([#1248](https://github.com/dhis2-sre/im-manager/issues/1248))
* fix casing
* grant access to HTTPScaledObject
* use the updated helm chart (version 0.29.0) which configures dhis2 using environment variables ([#1222](https://github.com/dhis2-sre/im-manager/issues/1222))
* add separate env var for enabling pretty print logs ([#1197](https://github.com/dhis2-sre/im-manager/issues/1197))
* switch to legacy repository ([#1209](https://github.com/dhis2-sre/im-manager/issues/1209))
* unlock before deleting ([#1195](https://github.com/dhis2-sre/im-manager/issues/1195))
* create LICENSE ([#1192](https://github.com/dhis2-sre/im-manager/issues/1192))
* add creat instance screen
* screenshots ([#1190](https://github.com/dhis2-sre/im-manager/issues/1190))
* bump helmfile and other binaries ([#1184](https://github.com/dhis2-sre/im-manager/issues/1184))
* generate change log

### Feat

* database description property ([#1233](https://github.com/dhis2-sre/im-manager/issues/1233))
* allow suspending
* user-owned databases ([#1183](https://github.com/dhis2-sre/im-manager/issues/1183))

### Fix

* use port-forwarding for MinIO backups ([#1226](https://github.com/dhis2-sre/im-manager/issues/1226))
* sort databases list ([#1196](https://github.com/dhis2-sre/im-manager/issues/1196))
* preload Instances when finding Deployments ([#1213](https://github.com/dhis2-sre/im-manager/issues/1213))
* saving a database ([#1179](https://github.com/dhis2-sre/im-manager/issues/1179))


<a name="v0.58.0"></a>
## [v0.58.0](https://github.com/dhis2-sre/im-manager/compare/v0.57.2...v0.58.0)

> 2025-08-09

### Chore

* fix createGroup script ([#1174](https://github.com/dhis2-sre/im-manager/issues/1174))
* increase proxy buffer size ([#1175](https://github.com/dhis2-sre/im-manager/issues/1175))
* include cluster
* sops

### Feat

* pretty log json ([#1164](https://github.com/dhis2-sre/im-manager/issues/1164))


<a name="v0.57.2"></a>
## [v0.57.2](https://github.com/dhis2-sre/im-manager/compare/v0.57.1...v0.57.2)

> 2025-07-22


<a name="v0.57.1"></a>
## [v0.57.1](https://github.com/dhis2-sre/im-manager/compare/v0.57.0...v0.57.1)

> 2025-07-14


<a name="v0.57.0"></a>
## [v0.57.0](https://github.com/dhis2-sre/im-manager/compare/v0.56.1...v0.57.0)

> 2025-07-10

### Chore

* generate change log


<a name="v0.56.1"></a>
## [v0.56.1](https://github.com/dhis2-sre/im-manager/compare/v0.56.0...v0.56.1)

> 2025-07-10

### Chore

* bump chart version
* generate change log

### Feat

* initial cluster model ([#1142](https://github.com/dhis2-sre/im-manager/issues/1142))


<a name="v0.56.0"></a>
## [v0.56.0](https://github.com/dhis2-sre/im-manager/compare/v0.55.2...v0.56.0)

> 2025-07-09

### Chore

* change chart name ([#1144](https://github.com/dhis2-sre/im-manager/issues/1144))

### Feat

* support DHIS2 with Embedded Tomcat ([#1145](https://github.com/dhis2-sre/im-manager/issues/1145))
* inspector ([#1138](https://github.com/dhis2-sre/im-manager/issues/1138))
* include lock when listing deployments ([#1143](https://github.com/dhis2-sre/im-manager/issues/1143))
* include User and Instance when retrieving the Lock ([#1141](https://github.com/dhis2-sre/im-manager/issues/1141))

### Fix

* save ([#1139](https://github.com/dhis2-sre/im-manager/issues/1139))

### Refactor

* add endpoints for updating deployment and deployment instance ([#1091](https://github.com/dhis2-sre/im-manager/issues/1091))


<a name="v0.55.2"></a>
## [v0.55.2](https://github.com/dhis2-sre/im-manager/compare/v0.55.1...v0.55.2)

> 2025-06-22

### Chore

* generate change log


<a name="v0.55.1"></a>
## [v0.55.1](https://github.com/dhis2-sre/im-manager/compare/v0.55.0...v0.55.1)

> 2025-06-20

### Chore

* generate changelog for v0.55.0 ([#1107](https://github.com/dhis2-sre/im-manager/issues/1107))

### Docs

* spelling

### Fix

* remote status ([#1126](https://github.com/dhis2-sre/im-manager/issues/1126))


<a name="v0.55.0"></a>
## [v0.55.0](https://github.com/dhis2-sre/im-manager/compare/v0.54.1...v0.55.0)

> 2025-05-30

### Chore

* bump sops and other versions ([#1099](https://github.com/dhis2-sre/im-manager/issues/1099))

### Feat

* stream uploads to s3 ([#1098](https://github.com/dhis2-sre/im-manager/issues/1098))


<a name="v0.54.1"></a>
## [v0.54.1](https://github.com/dhis2-sre/im-manager/compare/v0.54.0...v0.54.1)

> 2025-05-22

### Chore

* preload groups
* generate change log


<a name="v0.54.0"></a>
## [v0.54.0](https://github.com/dhis2-sre/im-manager/compare/v0.53.0...v0.54.0)

> 2025-05-20

### Feat

* dynamic namespaces ([#1087](https://github.com/dhis2-sre/im-manager/issues/1087))


<a name="v0.53.0"></a>
## [v0.53.0](https://github.com/dhis2-sre/im-manager/compare/v0.52.0...v0.53.0)

> 2025-05-09

### Chore

* add emis group ([#1085](https://github.com/dhis2-sre/im-manager/issues/1085))
* initial fs ([#1042](https://github.com/dhis2-sre/im-manager/issues/1042))

### Feat

* same site cookies ([#1067](https://github.com/dhis2-sre/im-manager/issues/1067))


<a name="v0.52.0"></a>
## [v0.52.0](https://github.com/dhis2-sre/im-manager/compare/v0.51.0...v0.52.0)

> 2025-03-07

### Chore

* im environment identifier ([#1026](https://github.com/dhis2-sre/im-manager/issues/1026))
* rename postgresql service to avoid issues with running instances ([#1021](https://github.com/dhis2-sre/im-manager/issues/1021))
* name service ([#1020](https://github.com/dhis2-sre/im-manager/issues/1020))
* opentelemetry ([#1014](https://github.com/dhis2-sre/im-manager/issues/1014))

### Ci

* bump postgres resource limits ([#1034](https://github.com/dhis2-sre/im-manager/issues/1034))
* move redis helm values to master config ([#1004](https://github.com/dhis2-sre/im-manager/issues/1004))
* add nodeSelector and tolerations for dedicated prod env node ([#917](https://github.com/dhis2-sre/im-manager/issues/917))

### Refactor

* use wait flag when running helmfile destroy ([#1027](https://github.com/dhis2-sre/im-manager/issues/1027))
* generic backup source and concrete MinIO implementation ([#1018](https://github.com/dhis2-sre/im-manager/issues/1018))


<a name="v0.51.0"></a>
## [v0.51.0](https://github.com/dhis2-sre/im-manager/compare/v0.50.1...v0.51.0)

> 2025-02-13


<a name="v0.50.1"></a>
## [v0.50.1](https://github.com/dhis2-sre/im-manager/compare/v0.50.0...v0.50.1)

> 2025-02-12

### Chore

* pin pg_restore to version 16 ([#999](https://github.com/dhis2-sre/im-manager/issues/999))


<a name="v0.50.0"></a>
## [v0.50.0](https://github.com/dhis2-sre/im-manager/compare/v0.49.0...v0.50.0)

> 2025-02-11

### Chore

* fix fs restore ([#998](https://github.com/dhis2-sre/im-manager/issues/998))
* fix imports
* bump versions ([#985](https://github.com/dhis2-sre/im-manager/issues/985))
* list custom DHIS2 configuration at the top of the file so user ([#988](https://github.com/dhis2-sre/im-manager/issues/988))
* bump default database chart version and default database version ([#980](https://github.com/dhis2-sre/im-manager/issues/980))
* don't set public property on anything but dhis2-core instances ([#976](https://github.com/dhis2-sre/im-manager/issues/976))

### Feat

* restore FS ([#981](https://github.com/dhis2-sre/im-manager/issues/981))
* custom dhis2 config ([#983](https://github.com/dhis2-sre/im-manager/issues/983))
* save file storage ([#974](https://github.com/dhis2-sre/im-manager/issues/974))

### Fix

* dhis.conf connection.schema should not be set ([#989](https://github.com/dhis2-sre/im-manager/issues/989))


<a name="v0.49.0"></a>
## [v0.49.0](https://github.com/dhis2-sre/im-manager/compare/v0.48.0...v0.49.0)

> 2025-01-13

### Chore

* pass public property in deploy script ([#968](https://github.com/dhis2-sre/im-manager/issues/968))

### Feat

* only encrypt sensitive parameters ([#970](https://github.com/dhis2-sre/im-manager/issues/970))
* instance details ([#967](https://github.com/dhis2-sre/im-manager/issues/967))


<a name="v0.48.0"></a>
## [v0.48.0](https://github.com/dhis2-sre/im-manager/compare/v0.47.0...v0.48.0)

> 2024-12-09

### Chore

* bump pgAdmin chart ([#965](https://github.com/dhis2-sre/im-manager/issues/965))


<a name="v0.47.0"></a>
## [v0.47.0](https://github.com/dhis2-sre/im-manager/compare/v0.46.0...v0.47.0)

> 2024-11-27

### Chore

* sort order


<a name="v0.46.0"></a>
## [v0.46.0](https://github.com/dhis2-sre/im-manager/compare/v0.45.0...v0.46.0)

> 2024-11-27

### Chore

* fix test


<a name="v0.45.0"></a>
## [v0.45.0](https://github.com/dhis2-sre/im-manager/compare/v0.43.0...v0.45.0)

> 2024-11-27

### Chore

* only list instances of type dhis2-core
* order public instances as stable, development and nightly


<a name="v0.43.0"></a>
## [v0.43.0](https://github.com/dhis2-sre/im-manager/compare/v0.44.0...v0.43.0)

> 2024-11-27


<a name="v0.44.0"></a>
## [v0.44.0](https://github.com/dhis2-sre/im-manager/compare/v0.42.0...v0.44.0)

> 2024-11-27

### Chore

* fix get latest tag
* use PAT


<a name="v0.42.0"></a>
## [v0.42.0](https://github.com/dhis2-sre/im-manager/compare/v0.41.0...v0.42.0)

> 2024-11-20


<a name="v0.41.0"></a>
## [v0.41.0](https://github.com/dhis2-sre/im-manager/compare/v0.40.0...v0.41.0)

> 2024-11-20

### Refactor

* run this workflow when the "Release" workflow is completed


<a name="v0.40.0"></a>
## [v0.40.0](https://github.com/dhis2-sre/im-manager/compare/v0.39.0...v0.40.0)

> 2024-11-15

### Chore

* tag


<a name="v0.39.0"></a>
## [v0.39.0](https://github.com/dhis2-sre/im-manager/compare/v0.38.0...v0.39.0)

> 2024-11-15

### Feat

* persistent storage using PVC ([#944](https://github.com/dhis2-sre/im-manager/issues/944))


<a name="v0.38.0"></a>
## [v0.38.0](https://github.com/dhis2-sre/im-manager/compare/v0.37.0...v0.38.0)

> 2024-11-14

### Chore

* git push --tags
* don't cat key


<a name="v0.37.0"></a>
## [v0.37.0](https://github.com/dhis2-sre/im-manager/compare/v0.36.0...v0.37.0)

> 2024-11-14

### Chore

* cat key
* specify gpg.format
* get latest tag
* start ssh agent
* start ssh agent
* ci release workflow ([#945](https://github.com/dhis2-sre/im-manager/issues/945))
* pass context to Gorm DEVOPS-397 ([#928](https://github.com/dhis2-sre/im-manager/issues/928))
* remove use of log.Fatal DEVOPS-394 ([#927](https://github.com/dhis2-sre/im-manager/issues/927))
* misc. things missing ([#925](https://github.com/dhis2-sre/im-manager/issues/925))

### Test

* remove commented out mock based tests ([#931](https://github.com/dhis2-sre/im-manager/issues/931))


<a name="v0.36.0"></a>
## [v0.36.0](https://github.com/dhis2-sre/im-manager/compare/v0.35.0...v0.36.0)

> 2024-09-30

### Chore

* revert d3492fd9a9d668c73c3f04e3d64d829bc5ff3293 ([#907](https://github.com/dhis2-sre/im-manager/issues/907))

### Docs

* add local development environment section to README ([#884](https://github.com/dhis2-sre/im-manager/issues/884))

### Feat

* add option for enabling query logging to dhis2 stack ([#901](https://github.com/dhis2-sre/im-manager/issues/901))
* public instances ([#885](https://github.com/dhis2-sre/im-manager/issues/885))

### Fix

* slog for destroy consumer

### Refactor

* log RabbitMQ correlationId ([#900](https://github.com/dhis2-sre/im-manager/issues/900))


<a name="v0.35.0"></a>
## [v0.35.0](https://github.com/dhis2-sre/im-manager/compare/v0.34.0...v0.35.0)

> 2024-08-23

### Fix

* filter out 'Evicted' pods when finding pod by instance id ([#864](https://github.com/dhis2-sre/im-manager/issues/864))


<a name="v0.34.0"></a>
## [v0.34.0](https://github.com/dhis2-sre/im-manager/compare/v0.33.0...v0.34.0)

> 2024-08-19

### Chore

* add implement group ([#860](https://github.com/dhis2-sre/im-manager/issues/860))


<a name="v0.33.0"></a>
## [v0.33.0](https://github.com/dhis2-sre/im-manager/compare/v0.32.0...v0.33.0)

> 2024-07-25

### Chore

* add missing return statement
* log name and group when destroying an instance due to TTL expiring ([#848](https://github.com/dhis2-sre/im-manager/issues/848))
* add error messages to assertions ([#849](https://github.com/dhis2-sre/im-manager/issues/849))


<a name="v0.32.0"></a>
## [v0.32.0](https://github.com/dhis2-sre/im-manager/compare/v0.31.1...v0.32.0)

> 2024-07-22

### Chore

* use sameSite none rather than lax ([#820](https://github.com/dhis2-sre/im-manager/issues/820))

### Feat

* remove token from response body ([#844](https://github.com/dhis2-sre/im-manager/issues/844))
* list public deployments ([#813](https://github.com/dhis2-sre/im-manager/issues/813))

### Fix

* close HTTP response body ([#818](https://github.com/dhis2-sre/im-manager/issues/818))


<a name="v0.31.1"></a>
## [v0.31.1](https://github.com/dhis2-sre/im-manager/compare/v0.31.0...v0.31.1)

> 2024-06-24

### Chore

* only try to parse access token if there is one ([#819](https://github.com/dhis2-sre/im-manager/issues/819))


<a name="v0.31.0"></a>
## [v0.31.0](https://github.com/dhis2-sre/im-manager/compare/v0.30.2...v0.31.0)

> 2024-06-22

### Chore

* allow manual deploy ([#816](https://github.com/dhis2-sre/im-manager/issues/816))
* fix/auth.sh ([#812](https://github.com/dhis2-sre/im-manager/issues/812))
* use sloglint context-only=scope ([#804](https://github.com/dhis2-sre/im-manager/issues/804))
* validate CORS config ([#797](https://github.com/dhis2-sre/im-manager/issues/797))
* get public from private key DEVOPS-259 ([#777](https://github.com/dhis2-sre/im-manager/issues/777))
* use single docker compose up ([#792](https://github.com/dhis2-sre/im-manager/issues/792))
* add "-pgadmin" to context path ([#767](https://github.com/dhis2-sre/im-manager/issues/767))
* remove comment

### Ci

* use gosec via golangci-lint ([#803](https://github.com/dhis2-sre/im-manager/issues/803))

### Feat

* add SSE streaming via /events ([#791](https://github.com/dhis2-sre/im-manager/issues/791))
* configure cookie same site mode ([#769](https://github.com/dhis2-sre/im-manager/issues/769))
* user scripts with cookie auth ([#766](https://github.com/dhis2-sre/im-manager/issues/766))
* remember me ([#724](https://github.com/dhis2-sre/im-manager/issues/724))

### Fix

* let skaffold update rabbitmq and redis on change ([#817](https://github.com/dhis2-sre/im-manager/issues/817))
* return request ID on status 500 ([#802](https://github.com/dhis2-sre/im-manager/issues/802))

### Test

* wait for users creation ([#778](https://github.com/dhis2-sre/im-manager/issues/778))


<a name="v0.30.2"></a>
## [v0.30.2](https://github.com/dhis2-sre/im-manager/compare/v0.30.1...v0.30.2)

> 2024-05-15

### Chore

* delete instance and not deployment ([#754](https://github.com/dhis2-sre/im-manager/issues/754))


<a name="v0.30.1"></a>
## [v0.30.1](https://github.com/dhis2-sre/im-manager/compare/v0.30.0...v0.30.1)

> 2024-05-14

### Chore

* use cookies in seed script ([#753](https://github.com/dhis2-sre/im-manager/issues/753))


<a name="v0.30.0"></a>
## [v0.30.0](https://github.com/dhis2-sre/im-manager/compare/v0.29.0...v0.30.0)

> 2024-05-14

### Chore

* log JSON so we can parse groups in Loki ([#752](https://github.com/dhis2-sre/im-manager/issues/752))
* log user if in context DEVOPS-393 ([#743](https://github.com/dhis2-sre/im-manager/issues/743))
* ban logging using log via linter ([#742](https://github.com/dhis2-sre/im-manager/issues/742))
* let Gorm log using slog DEVOPS-391 ([#741](https://github.com/dhis2-sre/im-manager/issues/741))
* let user implement LogValuer to hide secrets ([#740](https://github.com/dhis2-sre/im-manager/issues/740))
* replace all uses of log with slog DEVOPS-170 ([#739](https://github.com/dhis2-sre/im-manager/issues/739))
* log request id inside Gin context DEVOPS-170 ([#735](https://github.com/dhis2-sre/im-manager/issues/735))
* let gin log using slog ([#734](https://github.com/dhis2-sre/im-manager/issues/734))
* start counting from 1

### Feat

* add reset and request reset password endpoints ([#730](https://github.com/dhis2-sre/im-manager/issues/730))
* cookie based authentication ([#719](https://github.com/dhis2-sre/im-manager/issues/719))
* cleanup user scripts ([#729](https://github.com/dhis2-sre/im-manager/issues/729))
* secure auth cookie ([#723](https://github.com/dhis2-sre/im-manager/issues/723))

### Fix

* log slow query time typo
* update rabbitmq lib to fix connection issue DEVOPS-366 ([#732](https://github.com/dhis2-sre/im-manager/issues/732))

### Refactor

* remove code related to old way of deploying ([#708](https://github.com/dhis2-sre/im-manager/issues/708))


<a name="v0.29.0"></a>
## [v0.29.0](https://github.com/dhis2-sre/im-manager/compare/v0.28.1...v0.29.0)

> 2024-03-11

### Chore

* add user scripts for bulk deployment of DHIS2 and invoking of analytics generation ([#709](https://github.com/dhis2-sre/im-manager/issues/709))
* remove linked ([#701](https://github.com/dhis2-sre/im-manager/issues/701))
* ttl field in create deployment request should be int ([#704](https://github.com/dhis2-sre/im-manager/issues/704))
* add ttl field to deploy scripts ([#703](https://github.com/dhis2-sre/im-manager/issues/703))

### Refactor

* order users by email ([#707](https://github.com/dhis2-sre/im-manager/issues/707))
* deduplicate index names ([#706](https://github.com/dhis2-sre/im-manager/issues/706))


<a name="v0.28.1"></a>
## [v0.28.1](https://github.com/dhis2-sre/im-manager/compare/v0.28.0...v0.28.1)

> 2024-02-26

### Chore

* disable test ([#700](https://github.com/dhis2-sre/im-manager/issues/700))


<a name="v0.28.0"></a>
## [v0.28.0](https://github.com/dhis2-sre/im-manager/compare/v0.27.0...v0.28.0)

> 2024-02-22

### Chore

* update reset.sh to target deployment instances ([#699](https://github.com/dhis2-sre/im-manager/issues/699))
* update postgresql chart version ([#664](https://github.com/dhis2-sre/im-manager/issues/664))
* fix database slug
* update scripts to use deployments ([#694](https://github.com/dhis2-sre/im-manager/issues/694))
* update cluster service account role arn ([#688](https://github.com/dhis2-sre/im-manager/issues/688))
* make all stack parameters required ([#677](https://github.com/dhis2-sre/im-manager/issues/677))
* bump chart version
* update status.sh script
* use correct id
* join user when fetching deployment
* s/deployment/deployments/
* use default ttl
* lock database using DeploymentInstance
* use id rather than name
* add android group ([#618](https://github.com/dhis2-sre/im-manager/issues/618))
* bump DHIS2 and SL DB versions to the latest ([#600](https://github.com/dhis2-sre/im-manager/issues/600))
* inline struct instantiation
* use interface

### Ci

* fix e2e tests workflow ref ([#663](https://github.com/dhis2-sre/im-manager/issues/663))
* trigger im-web-client e2e tests workflow ([#625](https://github.com/dhis2-sre/im-manager/issues/625))

### Feat

* administrator will now see all databases and instances despite not actually being a group member ([#665](https://github.com/dhis2-sre/im-manager/issues/665))
* status for deployment instances ([#647](https://github.com/dhis2-sre/im-manager/issues/647))
* implement TTL and public flag for deployments ([#646](https://github.com/dhis2-sre/im-manager/issues/646))
* "save as" deployment instance ([#639](https://github.com/dhis2-sre/im-manager/issues/639))
* deployment instance logs ([#631](https://github.com/dhis2-sre/im-manager/issues/631))
* list deployments ([#634](https://github.com/dhis2-sre/im-manager/issues/634))
* reset deployment instance ([#633](https://github.com/dhis2-sre/im-manager/issues/633))
* restart deployment instance ([#632](https://github.com/dhis2-sre/im-manager/issues/632))
* encryption ([#630](https://github.com/dhis2-sre/im-manager/issues/630))
* add Name field to StackParameter ([#601](https://github.com/dhis2-sre/im-manager/issues/601))

### Fix

* deploy both core and pgAdmin ([#645](https://github.com/dhis2-sre/im-manager/issues/645))
* lower case parameter.Value json

### Refactor

* rename Name field to DisplayName in StackParameter ([#638](https://github.com/dhis2-sre/im-manager/issues/638))
* delete external downloads before deleting database ([#619](https://github.com/dhis2-sre/im-manager/issues/619))

### Test

* fix locks test ([#641](https://github.com/dhis2-sre/im-manager/issues/641))


<a name="v0.27.0"></a>
## [v0.27.0](https://github.com/dhis2-sre/im-manager/compare/v0.26.0...v0.27.0)

> 2023-11-22

### Chore

* remove log statement
* revert sops ([#599](https://github.com/dhis2-sre/im-manager/issues/599))
* adjust code to AWS S3 library update
* use aws go sdk v2 ([#591](https://github.com/dhis2-sre/im-manager/issues/591))
* bump dhis2-core memory resource requests ([#585](https://github.com/dhis2-sre/im-manager/issues/585))
* show error message
* update list of groups for prod env ([#567](https://github.com/dhis2-sre/im-manager/issues/567))

### Debug

* log stack parameters path

### Docs

* how to add a group ([#584](https://github.com/dhis2-sre/im-manager/issues/584))

### Fix

* groups order ([#563](https://github.com/dhis2-sre/im-manager/issues/563))

### Refactor

* return conflict if multiple pods are found ([#548](https://github.com/dhis2-sre/im-manager/issues/548))

### Reverts

* chore: show error message


<a name="v0.26.0"></a>
## [v0.26.0](https://github.com/dhis2-sre/im-manager/compare/v0.25.0...v0.26.0)

> 2023-11-02

### Chore

* implement user script for adding a pgAdmin instance to a deployment

### Feat

* show required stacks ([#554](https://github.com/dhis2-sre/im-manager/issues/554))
* stable rendering of groups


<a name="v0.25.0"></a>
## [v0.25.0](https://github.com/dhis2-sre/im-manager/compare/v0.24.0...v0.25.0)

> 2023-10-19

### Chore

* bump dhis2 stacks ingress max body size to 128m ([#528](https://github.com/dhis2-sre/im-manager/issues/528))
* update repository
* bump whoami-go chart version
* fix faulty commit
* add "design" group ([#527](https://github.com/dhis2-sre/im-manager/issues/527))
* update default chart version
* remove a lot of unused generated types ([#510](https://github.com/dhis2-sre/im-manager/issues/510))
* s/read/write/
* bump max upload size to 512MB ([#485](https://github.com/dhis2-sre/im-manager/issues/485))
* add missing hostname pattern
* initial attempt at deploying ([#467](https://github.com/dhis2-sre/im-manager/issues/467))

### Feat

* post slack message if e2e tests fails ([#530](https://github.com/dhis2-sre/im-manager/issues/530))
* initial instance status ([#512](https://github.com/dhis2-sre/im-manager/issues/512))
* prioritize parameters ([#513](https://github.com/dhis2-sre/im-manager/issues/513))
* destroy deployment instance ([#479](https://github.com/dhis2-sre/im-manager/issues/479))
* show user validated status ([#491](https://github.com/dhis2-sre/im-manager/issues/491))
* destroy deployment ([#478](https://github.com/dhis2-sre/im-manager/issues/478))

### Fix

* database id can be both string and uint
* linking stopped working with the new instance deployment strategy. This "hack" fixes it ([#474](https://github.com/dhis2-sre/im-manager/issues/474))

### Refactor

* support S3 folder/path structure with an optional db prefix ([#503](https://github.com/dhis2-sre/im-manager/issues/503))
* bump default dhis2 and dhis2-db stacks DATABASE_SIZE ([#517](https://github.com/dhis2-sre/im-manager/issues/517))

### Test

* deploy DHIS2. Works locally but not on GHA! ([#502](https://github.com/dhis2-sre/im-manager/issues/502))


<a name="v0.24.0"></a>
## [v0.24.0](https://github.com/dhis2-sre/im-manager/compare/v0.23.0...v0.24.0)

> 2023-09-12

### Chore

* make instance handler testable ([#464](https://github.com/dhis2-sre/im-manager/issues/464))
* remove stack parameter name ([#459](https://github.com/dhis2-sre/im-manager/issues/459))
* abstract new stack implementation using dto struct in the handler ([#458](https://github.com/dhis2-sre/im-manager/issues/458))
* move loader code to test ([#456](https://github.com/dhis2-sre/im-manager/issues/456))
* validate stack config has no cycles ([#449](https://github.com/dhis2-sre/im-manager/issues/449))
* source auth.sh in users scripts ([#450](https://github.com/dhis2-sre/im-manager/issues/450))
* bump golangci-lint version
* don't preload stack parameters ([#451](https://github.com/dhis2-sre/im-manager/issues/451))
* explicitly name Provider ParameterProvider ([#448](https://github.com/dhis2-sre/im-manager/issues/448))
* add parameter validation mechanism ([#446](https://github.com/dhis2-sre/im-manager/issues/446))
* validate consumed parameters are provided by required stacks DEVOPS-223 ([#440](https://github.com/dhis2-sre/im-manager/issues/440))
* declare all stacks ([#441](https://github.com/dhis2-sre/im-manager/issues/441))
* create map of parameter names to parameters ([#439](https://github.com/dhis2-sre/im-manager/issues/439))
* declare stacks in Go ([#435](https://github.com/dhis2-sre/im-manager/issues/435))
* gnomock update breaks build ([#438](https://github.com/dhis2-sre/im-manager/issues/438))
* gnomock update breaks build ([#436](https://github.com/dhis2-sre/im-manager/issues/436))
* ignore k8s dependencies updates to prevent build issues ([#431](https://github.com/dhis2-sre/im-manager/issues/431))
* bump versions
* enhance security ([#407](https://github.com/dhis2-sre/im-manager/issues/407))
* add docker to dependabot package ecosystem

### Feat

* deployments DEVOPS-223 ([#460](https://github.com/dhis2-sre/im-manager/issues/460))
* validate email ([#393](https://github.com/dhis2-sre/im-manager/issues/393))
* return all groups (also for administrators) and an empty list rather than null ([#408](https://github.com/dhis2-sre/im-manager/issues/408))

### Fix

* missing parameters ([#457](https://github.com/dhis2-sre/im-manager/issues/457))
* return 401 rather than 500 if refresh token has been purged

### Refactor

* update loader to assert statically defined stacks match wha… ([#445](https://github.com/dhis2-sre/im-manager/issues/445))
* move environment subdomain at the beggining of hostnames ([#417](https://github.com/dhis2-sre/im-manager/issues/417))
* add support for S3 compatible storage ([#392](https://github.com/dhis2-sre/im-manager/issues/392))
* merge optional and required parameters in to one type calle… ([#358](https://github.com/dhis2-sre/im-manager/issues/358))

### Test

* add instance integration test ([#465](https://github.com/dhis2-sre/im-manager/issues/465))


<a name="v0.23.0"></a>
## [v0.23.0](https://github.com/dhis2-sre/im-manager/compare/v0.21.0...v0.23.0)

> 2023-07-28

### Chore

* add note about downgrading libraries
* dummy
* downgrade
* pin alpha.1
* use master
* enable dependabot on PR's
* fix compile error
* add secret to workflow
* compile error which should trigger a slack message
* update readme
* change admin user email and password ([#343](https://github.com/dhis2-sre/im-manager/issues/343))
* add request error to context where missing
* fix escaping
* don't quote booleans
* bump version and use slug ([#315](https://github.com/dhis2-sre/im-manager/issues/315))
* add image pull policy integration ([#313](https://github.com/dhis2-sre/im-manager/issues/313))
* fix index
* fix swagger
* rename function
* fix Intellij warnings
* s/HOSTNAME/API_HOSTNAME/ for the sake of avoiding collisions when deploying from im-tooling
* remove custom reverse function

### Ci

* set S3 bucket based on environment/classification ([#348](https://github.com/dhis2-sre/im-manager/issues/348))

### Feat

* remove user from a group and return a group with its users ([#342](https://github.com/dhis2-sre/im-manager/issues/342))
* human-readable error messages when signing up ([#324](https://github.com/dhis2-sre/im-manager/issues/324))
* keep original creation date when saving a database
* public instances ([#314](https://github.com/dhis2-sre/im-manager/issues/314))
* instance description
* deployable/non-deployable groups ([#311](https://github.com/dhis2-sre/im-manager/issues/311))
* sort Docker tags (integration)
* sort Docker images (integration)

### Fix

* deployable has no effect when group is created
* remove "required" binding from Deployable bool field ([#341](https://github.com/dhis2-sre/im-manager/issues/341))
* update REFRESH_TOKEN_SECRET_KEY value for prod env ([#338](https://github.com/dhis2-sre/im-manager/issues/338))

### Refactor

* run instance-manager workflow on PR for feature branches ([#318](https://github.com/dhis2-sre/im-manager/issues/318))
* order by updatedAt
* sort instance by date of creation
* join group with instance
* check for gorm.ErrDuplicatedKey and return conflict for duplicated keys ([#312](https://github.com/dhis2-sre/im-manager/issues/312))
* filter by "supported" field to simplify seed script ([#307](https://github.com/dhis2-sre/im-manager/issues/307))

### Test

* test user, stack and database handler ([#275](https://github.com/dhis2-sre/im-manager/issues/275))

### Reverts

* chore: dummy


<a name="v0.21.0"></a>
## [v0.21.0](https://github.com/dhis2-sre/im-manager/compare/v0.20.0...v0.21.0)

> 2023-06-28

### Chore

* add missing environment variable
* remove jwks service... Completely! Again!
* remove jwks service... Completely!
* unexport signUpRequest
* document update user request and unexport it ([#304](https://github.com/dhis2-sre/im-manager/issues/304))
* remove jwks service
* load user along with the instance for the sake of showing who owns the instance client side

### Feat

* find all groups a user can access ([#305](https://github.com/dhis2-sre/im-manager/issues/305))

### Fix

* INSTANCE_HOST

### Refactor

* improve databases seed script ([#306](https://github.com/dhis2-sre/im-manager/issues/306))


<a name="v0.20.0"></a>
## [v0.20.0](https://github.com/dhis2-sre/im-manager/compare/v0.19.0...v0.20.0)

> 2023-06-23

### Chore

* json first and cascade delete externalDownload's ([#303](https://github.com/dhis2-sre/im-manager/issues/303))


<a name="v0.19.0"></a>
## [v0.19.0](https://github.com/dhis2-sre/im-manager/compare/v0.18.0...v0.19.0)

> 2023-06-23

### Chore

* bump chart version for dev
* update the chart version for im-group ([#302](https://github.com/dhis2-sre/im-manager/issues/302))
* move GroupsWithDatabases into the handler. It's not a database model but rather a DTO
* classify HOSTNAME as a system parameter
* remove kubernetes role resources but keep IM groups ([#298](https://github.com/dhis2-sre/im-manager/issues/298))
* s/interface{}/any/
* change expiration from time.Time to uint representing seconds until expiration ([#293](https://github.com/dhis2-sre/im-manager/issues/293))

### Fix

* whoami group hostname


<a name="v0.18.0"></a>
## [v0.18.0](https://github.com/dhis2-sre/im-manager/compare/v0.17.0...v0.18.0)

> 2023-06-22

### Chore

* camel case properties
* revert dependabot updates ([#296](https://github.com/dhis2-sre/im-manager/issues/296))
* temporary disable dependabot merge workflow ([#295](https://github.com/dhis2-sre/im-manager/issues/295))
* expose headers
* move comment to the line above the declaration since Github can't handle such within $GITHUB_ENV
* remove unused environment variables
* revert dependabot changes from PRs [#279](https://github.com/dhis2-sre/im-manager/issues/279) and [#280](https://github.com/dhis2-sre/im-manager/issues/280) ([#285](https://github.com/dhis2-sre/im-manager/issues/285))
* introduce hostname environment variable so we can control the database download url ([#277](https://github.com/dhis2-sre/im-manager/issues/277))
* remove individual stack parameters ([#269](https://github.com/dhis2-sre/im-manager/issues/269))
* revert dependabot changes from PRs [#265](https://github.com/dhis2-sre/im-manager/issues/265) and [#267](https://github.com/dhis2-sre/im-manager/issues/267) ([#268](https://github.com/dhis2-sre/im-manager/issues/268))
* fix import
* move middleware authentication into the middleware package

### Feat

* implement updating users' email and/or password ([#264](https://github.com/dhis2-sre/im-manager/issues/264))

### Refactor

* align all JSON responses to be in camelCase ([#276](https://github.com/dhis2-sre/im-manager/issues/276))


<a name="v0.17.0"></a>
## [v0.17.0](https://github.com/dhis2-sre/im-manager/compare/v0.16.0...v0.17.0)

> 2023-06-09

### Chore

* use parameter helper function ([#251](https://github.com/dhis2-sre/im-manager/issues/251))
* lower token expiration times
* remove apperror ([#259](https://github.com/dhis2-sre/im-manager/issues/259))
* add missing return statement
* ensure cluster configuration exists on the group
* remove redundant type conversion
* remove pvc delete commands as pvc are now automatically deleted on destroy
* don't return the stack when passing a pointer
* rename variables for consistency
* no clean before test
* remove logging
* don't return the instance when passing a pointer
* remove unused argument
* use master when calling our reusable GHA

### Feat

* implement deleting users by their id ([#257](https://github.com/dhis2-sre/im-manager/issues/257))
* implement listing of users for admins users ([#253](https://github.com/dhis2-sre/im-manager/issues/253))

### Fix

* save ([#260](https://github.com/dhis2-sre/im-manager/issues/260))
* ensure instance exists before locking database
* show correct path during helm deploy


<a name="v0.16.0"></a>
## [v0.16.0](https://github.com/dhis2-sre/im-manager/compare/v0.15.0...v0.16.0)

> 2023-05-26

### Docs

* document /integrations

### Feat

* delete pvc's when destroying an instance ([#249](https://github.com/dhis2-sre/im-manager/issues/249))

### Fix

* typo

### Refactor

* update KMS keys for IM helm chart and stacks secrets ([#246](https://github.com/dhis2-sre/im-manager/issues/246))
* match instance parameters with stack parameters before saving ([#242](https://github.com/dhis2-sre/im-manager/issues/242))


<a name="v0.15.0"></a>
## [v0.15.0](https://github.com/dhis2-sre/im-manager/compare/v0.14.0...v0.15.0)

> 2023-04-20

### Chore

* define database related routes in the database package ([#193](https://github.com/dhis2-sre/im-manager/issues/193))

### Feat

* Add resource requests env vars for core and DB [DEVOPS-214] ([#136](https://github.com/dhis2-sre/im-manager/issues/136))
* allow resuming paused instances  ([#197](https://github.com/dhis2-sre/im-manager/issues/197))

### Fix

* install postgresql-client rather than just copying the binaries (without the needed dependencies)
* allow im service restarts within skaffold deadline ([#196](https://github.com/dhis2-sre/im-manager/issues/196))


<a name="v0.14.0"></a>
## [v0.14.0](https://github.com/dhis2-sre/im-manager/compare/v0.13.1...v0.14.0)

> 2023-03-27

### Chore

* add readiness timeout parameter to deploy scripts
* expose readiness timeout as stack parameter
* enter correct folder
* read s3 bucket environment variable
* initial merge of the database manager ([#184](https://github.com/dhis2-sre/im-manager/issues/184))
* create profiles, for dev and prod, which will install im-group for each namespace IM should have access to ([#185](https://github.com/dhis2-sre/im-manager/issues/185))
* update source files
* granting all privileges on all tables isn't enough for flyway to work. In order to alter a table the user needs to own the table as well
* clean up seed script
* bump chart version
* fix swagger
* enable Redis cache invalidation
* remove pgAdmin parameters from user scripts
* remove pgadmin from the monolith stack. If pgadmin is needed it can be added by linking against the stack
* return an empty array rather than null ([#173](https://github.com/dhis2-sre/im-manager/issues/173))
* trim whitespace


<a name="v0.13.1"></a>
## [v0.13.1](https://github.com/dhis2-sre/im-manager/compare/v0.13.0...v0.13.1)

> 2023-02-24

### Chore

* return custom error... If err != nil


<a name="v0.13.0"></a>
## [v0.13.0](https://github.com/dhis2-sre/im-manager/compare/v0.12.0...v0.13.0)

> 2023-02-24

### Chore

* print swagger version for the sake of debugging
* remove unused make targets and cleanup docker compose file
* remove login script. It's no longer used as login is automated
* define interfaces where they're used ([#160](https://github.com/dhis2-sre/im-manager/issues/160))
* disable e2e on PR
* add flyway properties to deploy and update scripts
* s/INSTANCE_HOST/IM_HOST/g ([#157](https://github.com/dhis2-sre/im-manager/issues/157))
* bump core image version
* temp fix to deal with older httpie on Jenkins
* temp fix to deal with older httpie on Jenkins
* default ttl to 172800 seconds equal to 48 hours
* add the hostnamePattern to the monolith stack. Eventually we want to support multiple hostnames which is described in the jira task DEVOPS-220
* label stack pods with creation timestamp ([#153](https://github.com/dhis2-sre/im-manager/issues/153))
* remove the --create flag as it doesn't work when specifying a database using the -d flag
* bump workflow version
* fix go.mod
* fix swagger
* bump rabbitmq and postgresql versions
* add Docker Hub credentials to .env.example
* add instance manager service host variable to .env.example so the tests will succeed
* remove job client
* remove job service configuration
* remove job service configuration
* remove job runner client
* increase timeouts and "unroll" destroy.sh
* update parameter name to match what's defined in the stack
* only lookup source instance if an instance's name is passed as argument
* update arguments to be consistent with other scripts. Pattern: Group, name and then other arguments
* increase timeout for monolith test
* test presets
* don't decrypt when saving, some strings (IfNotPresent) appear encrypted but isn't
* only assert stack type if we're dealing with presets
* make expression more readable
* deploy and preset are mutually exclusive, and we already asserted that both of them aren't true
* asset deploy and preset isn't both true
* preserve the id of the preset for the sake of traceability
* specify "get" so older versions of httpie don't assume "post"
* implement /presets endpoint
* initial "Preset" implementation

### Docs

* query param "preset"

### Feat

* restart without downtime ([#170](https://github.com/dhis2-sre/im-manager/issues/170))

### Fix

* save instance parameters before updating
* order of arguments
* order of host variables
* .env file format

### Refactor

* specify HTTP request methods for httpie backward compatibility ([#155](https://github.com/dhis2-sre/im-manager/issues/155))

### Test

* remove unused variable
* print statement indicating successful completion
* echo commands
* cleanup any lingering instances ([#159](https://github.com/dhis2-sre/im-manager/issues/159))


<a name="v0.12.0"></a>
## [v0.12.0](https://github.com/dhis2-sre/im-manager/compare/v0.11.0...v0.12.0)

> 2022-11-24

### Feat

* Add Liveness probe timeoutSeconds as opt param to stacks and scripts ([#134](https://github.com/dhis2-sre/im-manager/issues/134))


<a name="v0.11.0"></a>
## [v0.11.0](https://github.com/dhis2-sre/im-manager/compare/v0.10.0...v0.11.0)

> 2022-11-23

### Feat

* Add optional Redis release to dhis2 stack ([#133](https://github.com/dhis2-sre/im-manager/issues/133))


<a name="v0.10.0"></a>
## [v0.10.0](https://github.com/dhis2-sre/im-manager/compare/v0.8.0...v0.10.0)

> 2022-11-07

### Chore

* allow a body size of 8m when uploading files through the ingress controller
* Add helm data for radnov env ([#125](https://github.com/dhis2-sre/im-manager/issues/125))
* sleep before http request
* implement update-whoami.sh
* expose IMAGE_TAG and IMAGE_PULL_POLICY as optional stack parameters
* only run on push to master
* fix pipeline
* bump chart and docker versions
* expose the environment variable "DHIS2_HOME" as an optional stack parameter with its default value set to "/opt/dhis2"
* configure CORS for browser based clients
* expose CHART_VERSION
* bump default dhis2/core version to 0.12.0
* add flyway properties
* use common labels
* bump chart version
* bump chart and default database version
* add IMAGE_REPOSITORY environment variable
* consistently label all stack resources ([#106](https://github.com/dhis2-sre/im-manager/issues/106))
* move kubernetes domain into kube service ([#104](https://github.com/dhis2-sre/im-manager/issues/104))
* update swagger
* remove redundant operations in restart ([#101](https://github.com/dhis2-sre/im-manager/issues/101))

### Feat

* Add user script for updating dhis2 stack instances [DEVOPS-208]
* allow pausing instances ([#105](https://github.com/dhis2-sre/im-manager/issues/105))
* user scripts auto login
* implement FindStack on the client

### Fix

* add missing quotes

### Refactor

* expose DATABASE_SIZE in user script
* make CHART_VERSION optional rather than required
* reuse scripts
* unexport "client" property


<a name="v0.8.0"></a>
## [v0.8.0](https://github.com/dhis2-sre/im-manager/compare/v0.7.0...v0.8.0)

> 2022-09-04

### Feat

* Go client

### Refactor

* use version 13 of Postgresql by default


<a name="v0.7.0"></a>
## [v0.7.0](https://github.com/dhis2-sre/im-manager/compare/v0.6.3...v0.7.0)

> 2022-08-24

### Chore

* scripts should deploy latest default dhis2 image ([#86](https://github.com/dhis2-sre/im-manager/issues/86))
* use curl rather than http to avoid extra newline in output
* remove unused docs code
* create stack in one transaction ([#75](https://github.com/dhis2-sre/im-manager/issues/75))
* separate parsing from CRUD ([#74](https://github.com/dhis2-sre/im-manager/issues/74))
* use startup probe for slow DHIS2 startup DEVOPS-180 ([#66](https://github.com/dhis2-sre/im-manager/issues/66))
* move kubernetes/helmfile interfaces to consumer ([#62](https://github.com/dhis2-sre/im-manager/issues/62))
* do not call os.Exit in loader ([#65](https://github.com/dhis2-sre/im-manager/issues/65))
* dockerignore scripts, mardown, ... ([#63](https://github.com/dhis2-sre/im-manager/issues/63))
* clarify error came from creating client
* adopt more go idioms in helmfile ([#59](https://github.com/dhis2-sre/im-manager/issues/59))
* remove unused receiver of decrypt ([#60](https://github.com/dhis2-sre/im-manager/issues/60))
* remove superfluous comments
* Bump reusable cicd workflow version
* add radnov env helm data
* Add sops files with stack parameters for dhis2-core and dhis2-db
* Update dhis2-db stack to match dhis2
* update stack parameters metadata list
* Update dhis2 chart repo for dhis2* stacks ([#40](https://github.com/dhis2-sre/im-manager/issues/40))
* update transitive deps ([#34](https://github.com/dhis2-sre/im-manager/issues/34))
* use usual Go prefix New instead of Provide
* use inputDigest as tag policy so the developers aren't overwriting each other's images
* log success right after deletion
* log more details on error in ttlDestroyConsumer
* update im-user client to v0.7.4
* update im-user client to v0.7.3

### Feat

* download entire database dump to disk before importing, so it's possible to run pg_restore in parallel
* retrieve the current user with groups from the context
* scale whoami-go
* restart instances
* whoami deploy script with TTL parameter
* use service account when deleting an instance as a result of co… ([#31](https://github.com/dhis2-sre/im-manager/issues/31))

### Fix

* "curl: (23) Failure writing output to destination"
* missing generated docs, parameter isn't documented if struct property is "_"
* httpie is making a POST instead of a GET ([#69](https://github.com/dhis2-sre/im-manager/issues/69))
* do not discard error on writing kubeconfig ([#64](https://github.com/dhis2-sre/im-manager/issues/64))
* ensure kubeconfig is closed in case write fails ([#61](https://github.com/dhis2-sre/im-manager/issues/61))
* symlink
* "psql: fe_sendauth: no password supplied"
* nack TTL messages containing invalid JSON ([#57](https://github.com/dhis2-sre/im-manager/issues/57))
* "psql: fe_sendauth: no password supplied"
* "psql: fe_sendauth: no password supplied"
* remove DATABASE_NAME argument
* not able to extract multiple requiredEnv one the same line
* go mod tidy
* aws role for env ivo
* missing required DATABASE_MANAGER_URL parameter in env ivo

### Refactor

* use 4 jobs
* use [[
* use external url for database manager
* remove hardcoded port
* suppress word splitting warning
* move stack parameters one folder level up
* install Postgresql extensions pg_trgm and btree_gin by default
* return a pointer
* return error (instead of log.Fatal)
* rename variables
* test handler.GetUserFromContext
* add todo comment
* expose IMAGE_PULL_POLICY
* update user scripts ([#77](https://github.com/dhis2-sre/im-manager/issues/77))
* shorten doc id's, so they match function name
* merge FindByIdWithParameters and FindById into FindById
* restart an instance by issuing a http put method rather than a post
* remove endpoint POST /instances/:id and just use PUT /instances/:id instead, no reason to have both
* add comment detailing the user behind id 405
* symlink seed script from stacks/dhis2-db to stacks/dhis2
* use exec_psql
* wrap the psql command in a function
* use the "-d" flag of psql to specify the database rather than adding it as last argument
* assert instance is writable by current user
* s/accessToken/token/
* ack even if instance isn't found. Although a rare case, a user could delete an instance before the message is read from the queue ([#33](https://github.com/dhis2-sre/im-manager/issues/33))
* create engine without wire ([#30](https://github.com/dhis2-sre/im-manager/issues/30))
* add stack specific parameters for "ivo" environment


<a name="v0.6.3"></a>
## [v0.6.3](https://github.com/dhis2-sre/im-manager/compare/v0.6.2...v0.6.3)

> 2022-06-08

### Chore

* sync service account passwords with the ones defined on im-user


<a name="v0.6.2"></a>
## [v0.6.2](https://github.com/dhis2-sre/im-manager/compare/v0.6.1...v0.6.2)

> 2022-06-08

### Chore

* add eu-central-1 key for dhis2 stack parameters encryption


<a name="v0.6.1"></a>
## [v0.6.1](https://github.com/dhis2-sre/im-manager/compare/v0.6.0...v0.6.1)

> 2022-06-08

### Chore

* use updated rabbitmq consumer DEVOPS-130 ([#17](https://github.com/dhis2-sre/im-manager/issues/17))
* update service account arns
* encrypt parameters using key found in eu-central-1 as well as eu-north-1
* use updated rabbitmq consumer DEVOPS-130 ([#17](https://github.com/dhis2-sre/im-manager/issues/17))
* instance-queue moved to rabbitmq

### Ci

* increase sleep in smoke-test

### Fix

* Revert "chore: use updated rabbitmq consumer DEVOPS-130 ([#17](https://github.com/dhis2-sre/im-manager/issues/17))"
* Revert "fix: retry establishing rabbitmq connection on startup"
* Revert "fix: increase attempts to connect to RabbitMQ"
* increase attempts to connect to RabbitMQ
* retry establishing rabbitmq connection on startup


<a name="v0.6.0"></a>
## [v0.6.0](https://github.com/dhis2-sre/im-manager/compare/v0.5.11...v0.6.0)

> 2022-06-02

### Chore

* add missing environment variable
* use latest queue lib before DEVOPS-130
* bump CI/CD workflow
* remove deprecated TODO comment
* add environment ivo
* go mod tidy
* go mod tidy
* use make swagger in hook
* goimports is gofmt+
* Revert "fix: swagger spec"
* validate swagger
* validate swagger
* add swagger-check to make
* make sure hooks environments are also reinstalled
* bump workflow version
* use golangci-lint hook directly
* install pre-commit on init
* pre-commit hook for commit message
* bump workflow version
* bump workflow version
* bump workflow version
* show diff of linters on failure
* auto-update dependencies
* bump workflow version
* run linting last
* init direnv using make as well
* expose pre-commit via make
* add pre-commit hooks
* bump workflow version
* lint using golangci-lint on GitHub
* pin cicd workflow

### Ci

* add codeql scanning
* use squash when merging dependabot PRs
* bump workflow
* bump dependabot workflow
* add dependabot auto-merge workflow

### Feat

* initial stack specific parameters ([#22](https://github.com/dhis2-sre/im-manager/issues/22))
* identify stack parameters by their name rather than an autogenerated id which is likely to change across environments or by updates to the stack
* pgAdmin
* expose Docker image repository as an optional stack parameter (IMAGE_REPOSITORY), defaults to "core"
* validate instance name against dns_rfc1035_label
* add optional "selector" parameter to the Handler.Logs endpoint, so we can stream logs from other pods beside the main instance

### Fix

* typo
* fix the regexp so the return value of requiredEnv can be piped to other functions
* commit-msg hooks are not installed by default
* helm name
* add missing return statement
* add missing return statement
* only fail swagger-check if swagger.yml changed
* swagger spec
* installing go-swagger
* swagger spec
* go mod tidy
* go imports
* timeout might be due to dependency downloads
* align lint go version with go.mod

### Refactor

* add prod namespaces
* add prod parameters for the dhis2 stack
* update stack id to match the whoami-go stack on dev
* update hello.sh to parameter by name rather than id
* s/stackParameterId/stackParameter/
* rename ID to Name
* initialize database id with 1
* inline struct creation
* pass just the instance id rather than the whole object
* rename parameter
* list CHART_VERSION and CHART_VERSION_PARAMETER_ID at the top of the deploy script so it's easier to update
* add missing value
* add liveness and readiness parameters
* list parameter id along with their values for ease of updating
* add IM_ACCESS_TOKEN to system parameters
* s/token/accessToken/ and s/IM_TOKEN/IM_ACCESS_TOKEN/ so there's no doubt what kind of token we're dealing with
* bump default chart version
* print warning if the environment we're trying to inject isn't found
* use correct role for the service account
* update the dhis2-core and dhis2-db stacks, so they include the updates found in the dhis2 stack
* remove redundant conversion
* bump Go version
* bump Alpine version
* use entrypoint rather than cmd
* use correct type (string) in "docs" struct
* use an else statement, so we're not formatting the label selector twice
* assert selector is either "" or "data" and nothing else
* rename the reader variable (readCloser) to r, so we're following the conventions of the standard library

### Reverts

* chore: validate swagger

### Pull Requests

* Merge pull request [#9](https://github.com/dhis2-sre/im-manager/issues/9) from dhis2-sre/dependabot-go_modules-gorm.io-gorm-1.23.5
* Merge pull request [#8](https://github.com/dhis2-sre/im-manager/issues/8) from dhis2-sre/dependabot-go_modules-gorm.io-driver-postgres-1.3.5
* Merge pull request [#6](https://github.com/dhis2-sre/im-manager/issues/6) from dhis2-sre/dependabot-go_modules-k8s.io-client-go-0.25.0-alpha.0
* Merge pull request [#5](https://github.com/dhis2-sre/im-manager/issues/5) from dhis2-sre/dependabot-go_modules-k8s.io-api-0.25.0-alpha.0
* Merge pull request [#3](https://github.com/dhis2-sre/im-manager/issues/3) from dhis2-sre/DEVOPS-133
* Merge pull request [#4](https://github.com/dhis2-sre/im-manager/issues/4) from dhis2-sre/lint
* Merge pull request [#1](https://github.com/dhis2-sre/im-manager/issues/1) from dhis2-sre/feature-no-deploy/stream-any-im-logs


<a name="v0.5.11"></a>
## [v0.5.11](https://github.com/dhis2-sre/im-manager/compare/v0.5.10...v0.5.11)

> 2022-04-19

### Chore

* Change absolute seed URL for prod env [DEVOPS-102]

### Feat

* stream database directly into pg_restore or into gunzip and then psql
* expose JAVA_OPTS as an optional stack parameter, defaults to ""

### Fix

* make folder before trying to use it
* use correct variable for database name
* use random id rather than $$ (pid) (which isn't thread safe)
* RabbitMQ password for rest of the environments
* RabbitMQ password

### Refactor

* use ParseUint rather than Atoi for the sake of stricter parsing
* bump default chart version
* rename variables, they don't refer to folders
* make the script more verbose, so it's easier to see progress despite lack of output from commands

### Pull Requests

* Merge pull request [#2](https://github.com/dhis2-sre/im-manager/issues/2) from dhis2-sre/feature-no-deploy/seed-sql-and-pgc


<a name="v0.5.10"></a>
## [v0.5.10](https://github.com/dhis2-sre/im-manager/compare/v0.5.5...v0.5.10)

> 2022-04-11

### Fix

* Only change the ownership of generate_uid() func [DEVOPS-102]
* Use func names with argument signatures to change ownership [DEVOPS-102]


<a name="v0.5.5"></a>
## [v0.5.5](https://github.com/dhis2-sre/im-manager/compare/v0.5.6...v0.5.5)

> 2022-04-11

### Fix

* Use func names with argument signatures to change ownership [DEVOPS-102]


<a name="v0.5.6"></a>
## [v0.5.6](https://github.com/dhis2-sre/im-manager/compare/v0.5.9...v0.5.6)

> 2022-04-11

### Fix

* Use func names with argument signatures to change ownership [DEVOPS-102]


<a name="v0.5.9"></a>
## [v0.5.9](https://github.com/dhis2-sre/im-manager/compare/v0.5.8...v0.5.9)

> 2022-04-11

### Fix

* Use func names with argument signatures to change ownership [DEVOPS-102]


<a name="v0.5.8"></a>
## [v0.5.8](https://github.com/dhis2-sre/im-manager/compare/v0.5.7...v0.5.8)

> 2022-04-11

### Fix

* Use func names with argument signatures to change ownership [DEVOPS-102]


<a name="v0.5.7"></a>
## [v0.5.7](https://github.com/dhis2-sre/im-manager/compare/v0.5.3...v0.5.7)

> 2022-04-11

### Fix

* Use func names with argument signatures to change ownership [DEVOPS-102]


<a name="v0.5.3"></a>
## [v0.5.3](https://github.com/dhis2-sre/im-manager/compare/v0.5.4...v0.5.3)

> 2022-04-08


<a name="v0.5.4"></a>
## [v0.5.4](https://github.com/dhis2-sre/im-manager/compare/v0.5.2...v0.5.4)

> 2022-04-08

### Fix

* Change functions owner to dhis user when seeding [DEVOPS=102]


<a name="v0.5.2"></a>
## [v0.5.2](https://github.com/dhis2-sre/im-manager/compare/v0.5.1...v0.5.2)

> 2022-04-07

### Fix

* Update seed URL for prod env [DEVOPS-102]

### Refactor

* Add readiness probe delay parameter and update IDs [DEVOPS-102]
* Add explicit get method to destroy script [DEVOPS-102]
* Substitute seed path var with database id in deploy script [DEVOPS-102]
* Remove extra --check-status option and add explicit get [DEVOPS-102]


<a name="v0.5.1"></a>
## [v0.5.1](https://github.com/dhis2-sre/im-manager/compare/v0.5.0...v0.5.1)

> 2022-04-04

### Chore

* remove debug log statement
* "revert" previous commit, so it's only deployed to prod

### Feat

* expose database size as optional stack parameter

### Fix

* RabbitMQ password
* make default value a string value

### Refactor

* rewrite seed script with support for unzipped pgc files
* use $POSTGRESQL_VOLUME_DIR rather than hardcoded path
* quote variables to suppress Intellij warnings
* download seed data to mounted volume rather than root to avoid disk pressure on the node
* set ownership of tables, sequences and views to $DATABASE_USERNAME for both SQL and PGC format


<a name="v0.5.0"></a>
## [v0.5.0](https://github.com/dhis2-sre/im-manager/compare/v0.4.0...v0.5.0)

> 2022-03-29

### Chore

* hardcode database service host in seed script to prod. The intention of this commit is that we'll deploy it to prod and then "revert" for dev


<a name="v0.4.0"></a>
## [v0.4.0](https://github.com/dhis2-sre/im-manager/compare/v0.3.0...v0.4.0)

> 2022-03-24

### Chore

* cache build
* show go mod downloads
* add data for a new environment (tons) intended to be used by an individual developer
* increase initial health check delay
* bump default chart version for the DHIS2 stack

### Docs

* add todo comments
* update readme file with a reference to the main docs

### Feat

* expose initial health probe delay as stack parameters
* encrypt instance parameters upon deploy
* ensure root FS is RO, app isn't running as root, app is running as user with uid 405 and gid 100

### Fix

* change ownership of sequences and views as well as tables
* too short key, crypto/aes: invalid key size 8
* missing environment variable
* /logs return a stream of logs rather than an instance

### Refactor

* bump version of docker image binaries
* lower initial health probe delay to 5 minutes
* seed from dev WIP
* print database host (for debugging purposes)
* s/.cluster.local//
* use environment variables rather than hardcoded values
* change ownership of all tables within our database. For the sake of flexibility we stopped dumping the database with a specific owner. Since we're importing using the "postgres" but connecting with "dhis" we need to set ownership to "dhis"
* s/.cluster.local//
* expose database credentials as stack parameters
* reverse order of releases so PostgreSQL and RabbitMQ will be installed before the application itself
* increase resource constraints
* specify resource constraints for Postgresql
* increase initial health probe delays. In a busy cluster 180 seconds wasn't enough to start DHIS2
* expose flyway parameters as stack parameters
* ensure curl fails if it's unable to download the database
* bump Postgresql version on the DHIS2 stack
* extract hardcoded values
* specify more sensible resource definitions
* specify resources
* group hacky code together


<a name="v0.3.0"></a>
## [v0.3.0](https://github.com/dhis2-sre/im-manager/compare/v0.2.0...v0.3.0)

> 2022-02-24

### Chore

* remove redundant scripts. dhis2-create.sh and dhis2-deploy.sh are similar

### Feat

* script implementing a more complete use case. Covering create, deploy, stream logs and destroy
* scripts for creating and deploying a whoami-go instance
* script for finding an instance by its name (and group name)
* script finding an instance by its id
* add scripts for listing all stacks and a single stack by its id

### Refactor

* parameterize hardcoded values
* specify versions for RabbitMQ and PostgreSQL
* update passwords and yaml structure match new versions


<a name="v0.2.0"></a>
## [v0.2.0](https://github.com/dhis2-sre/im-manager/compare/v0.1.0...v0.2.0)

> 2022-02-15

### Chore

* update user scripts
* update environment data
* don't fail if spec doesn't exists
* update make (and friends) with configuration for RabbitMQ and JWKS

### Docs

* comment all handlers
* fix description
* remove deprecated warning

### Feat

* add job runner stack
* implement save instance endpoint
* implement various user scripts
* separate instance creation and deployment
* add swagger spec to final stage of docker image
* use service account to request group from im-user when destroying instance via ttl-destroy event
* serve ReDoc documentation from /docs
* list instances by groups
* authorize
* consume ttl-destroy events (working edition)
* consume ttl-destroy events

### Fix

* chart path
* can write, needs both instance ownership and group membership
* use relative swagger spec reference
* use a token which won't expire for another 100 years
* ensure requests are aborted in case of errors
* JWKS host
* helm prod data

### Refactor

* s/launch/deploy/ - for the sake of consistency
* pass token to helmfileService.Destroy
* change make target name for building and pushing docker images
* pass access token all the way to HelmfileService
* authenticate "stacks" end points
* move swagger definitions into the "health" package
* grant get, create, patch and delete access to jobs
* split DHIS2 stack into separate application and database stack
* lowercase error messages
* implement user scripts for creating and deploying DHIS2
* misc. minor updates
* only try to destroy instance if the entity has a deploy log
* deploy scripts accept group name (and uses new endpoint)
* s/launch.sh/deploy.sh/
* don't print group id
* lower case error message
* test and make code more readable
* add default username and password for the user service
* bump user service version
* update jwks and add token to README.md
* use jwt.ParseRequest rather than manually extracting the token from the http authentication header
* s/im-users/im-user/
* add RabbitMQ values for prod and dev
* use go-swagger rather than swaggo (and go-swagger)
* remove redundant uint type conversions
* authenticate instance.NameToId since it relies on having a user on the context
* use authenticated client to access user service
* launch RabbitMQ using Skaffold
* let skaffold timeout after 2 minutes which should be plenty of time to boot given the current state of the application
* remove play from groups. Just a single group is need for development and every group listed needs to be "manually" created in the cluster

### Style

* remove newline
* remove newline
* lower case error message

### Test

* remove redundant properties
* rename function
* remove redundant test


<a name="v0.1.0"></a>
## v0.1.0

> 2021-12-15

### Chore

* define helm values and secrets for dev and prod environments
* fix CI/CD input variables
* remove redis... again
* define .env file for prod service
* add .env example file
* initial CI/CD
* implement cluster-dev target
* install Postgresql using Skaffold
* initial helm chart
* initial commit

### Feat

* find instance by id, delete and stream logs
* decrypt Kubernetes config
* authorize user before creating instance
* launch instance (WIP)
* load stacks on boot
* stack handler (FindAll and FindById) and friends (config, di, routing, storage, health)

### Refactor

* use environment variable ENVIRONMENT
* define environment variable BASE_PATH separately since .Values.basePath can't default to / when used for health checks
* rename target to match what's expected by GHA reusable workflow
* add missing environment variables
* rename targets to match what's expected by GHA reusable workflow
* implement smoke-test target
* remove redis
* remove log statement
* use the chown option of the copy command so helm has write access to /app/stacks/*/{.config/,.cache/}
* configure skaffold to create namespace if needed
* remove aws cli since it's not used
* configure chart to be as generic as possible

