#!/usr/bin/env bash

set -xeuo pipefail

INSTANCE_HOST_DEPLOY=https://whoami.im.dev.test.c.dhis2.org
GROUP=whoami

# Whoami
INSTANCE_NAME=whoami-test
./deploy-whoami.sh $GROUP $INSTANCE_NAME
sleep 3
kubectl wait --for=condition=available --timeout=30s --namespace $GROUP deployment/$INSTANCE_NAME-whoami-go
http --check-status "$INSTANCE_HOST_DEPLOY/$INSTANCE_NAME"
./destroy.sh $GROUP $INSTANCE_NAME

# Monolith
INSTANCE_NAME=monolith-test
./deploy-dhis2.sh $GROUP $INSTANCE_NAME
sleep 3
kubectl wait --for=condition=available --timeout=180s --namespace $GROUP deployment/$INSTANCE_NAME-core
sleep 3
http --check-status --follow "$INSTANCE_HOST_DEPLOY/$INSTANCE_NAME"
./destroy.sh $GROUP $INSTANCE_NAME
kubectl delete pvc --namespace $GROUP data-$INSTANCE_NAME-database-postgresql-0

# Database and core
INSTANCE_NAME=db-and-core-test
./deploy-dhis2-db.sh $GROUP $INSTANCE_NAME
./deploy-dhis2-core.sh $GROUP $INSTANCE_NAME $INSTANCE_NAME-core
sleep 3
kubectl rollout status --watch --timeout=300s --namespace $GROUP statefulset/$INSTANCE_NAME-database-postgresql
kubectl wait --for=condition=available --timeout=300s --namespace $GROUP deployment/$INSTANCE_NAME-core
sleep 3
http --check-status --follow "$INSTANCE_HOST_DEPLOY/$INSTANCE_NAME-core"
./destroy.sh $GROUP "$INSTANCE_NAME-core"
./destroy.sh $GROUP "$INSTANCE_NAME"
kubectl delete pvc --namespace $GROUP data-$INSTANCE_NAME-database-postgresql-0
