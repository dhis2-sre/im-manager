# User scripts

Please see https://github.com/dhis2-sre/im-doc/tree/master/api#getting-started for an in-depth introduction to the user scripts accompanying each service

# Examples

## Login
```sh
export ACCESS_TOKEN="" && eval $(./login.sh) && echo $ACCESS_TOKEN | jwt
```

## Hello, World!
```sh
./hello.sh whoami hello
```

## DB
```sh
export MYID=tons-db-1 && ./deploy-dhis2-db.sh whoami $MYID; read && ./destroy.sh whoami $MYID
```

## Core
```sh
export MYID=tons-core-1 && ./deploy-dhis2-core.sh whoami tons-db-1 $MYID; read && ./destroy.sh whoami $MYID
```

## pgAdmin
```sh
export MYID=tons-pgadmin-1 && ./deploy-pgadmin.sh whoami tons-db-1 $MYID; read && ./destroy.sh whoami $MYID
```

## WhoAmI
```sh
export MYID=who-1 && ./whoami-create.sh whoami $MYID; read && ./whoami-deploy-existing.sh whoami $MYID && read && ./destroy.sh whoami $MYID
```

## Spawn 5 "your-instances"
```sh
./stress.sh whoami 5 your-instances
```

## Destroy all "your-instances"
```sh
./destroy.sh whoami $(./list.sh | jq -r '.[].Instances[].Name' | grep your-instances)
```
