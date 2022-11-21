#!/usr/bin/env bash

set -xeuo pipefail

INSTANCE_HOST_DEPLOY=https://whoami.im.dev.test.c.dhis2.org
GROUP=whoami

# Whoami
INSTANCE_NAME=test-whoami
./deploy-whoami.sh $GROUP $INSTANCE_NAME
sleep 3
kubectl wait --for=condition=available --timeout=30s --namespace $GROUP deployment/$INSTANCE_NAME-whoami-go
sleep 3
http --check-status "$INSTANCE_HOST_DEPLOY/$INSTANCE_NAME"
./destroy.sh $GROUP $INSTANCE_NAME

# Whoami Preset
INSTANCE_NAME=test-whoami-preset
./deploy-whoami-preset.sh $GROUP $INSTANCE_NAME-preset
sleep 3
./deploy-whoami-from-preset.sh whoami $INSTANCE_NAME $INSTANCE_NAME-preset
sleep 3
kubectl wait --for=condition=available --timeout=30s --namespace $GROUP deployment/$INSTANCE_NAME-whoami-go
sleep 3
http --check-status "$INSTANCE_HOST_DEPLOY/$INSTANCE_NAME"
./destroy.sh $GROUP $INSTANCE_NAME $INSTANCE_NAME-preset

# Monolith
INSTANCE_NAME=test-monolith
./deploy-dhis2.sh $GROUP $INSTANCE_NAME
sleep 3
kubectl wait --for=condition=available --timeout=180s --namespace $GROUP deployment/$INSTANCE_NAME-core
sleep 3
http --check-status --follow "$INSTANCE_HOST_DEPLOY/$INSTANCE_NAME"
./destroy.sh $GROUP $INSTANCE_NAME
kubectl delete pvc --namespace $GROUP data-$INSTANCE_NAME-database-postgresql-0

# Database and core
INSTANCE_NAME=test-db-and-core
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

# Database preset and core
INSTANCE_NAME=test-db-preset
./deploy-dhis2-db-preset.sh $GROUP $INSTANCE_NAME-preset
./deploy-dhis2-db-from-preset.sh whoami $INSTANCE_NAME $INSTANCE_NAME-preset
./deploy-dhis2-core.sh $GROUP $INSTANCE_NAME $INSTANCE_NAME-core
sleep 3
kubectl rollout status --watch --timeout=300s --namespace $GROUP statefulset/$INSTANCE_NAME-database-postgresql
kubectl wait --for=condition=available --timeout=300s --namespace $GROUP deployment/$INSTANCE_NAME-core
sleep 3
http --check-status --follow "$INSTANCE_HOST_DEPLOY/$INSTANCE_NAME-core"
./destroy.sh $GROUP $INSTANCE_NAME-core $INSTANCE_NAME-preset $INSTANCE_NAME
kubectl delete pvc --namespace $GROUP data-$INSTANCE_NAME-database-postgresql-0

# Database and core from preset
INSTANCE_NAME=test-db-and-core-preset
./deploy-dhis2-db.sh $GROUP $INSTANCE_NAME-db
./deploy-dhis2-preset.sh $GROUP $INSTANCE_NAME-preset
./deploy-dhis2-from-preset.sh $GROUP $INSTANCE_NAME $INSTANCE_NAME-preset
sleep 3
kubectl rollout status --watch --timeout=300s --namespace $GROUP statefulset/$INSTANCE_NAME-db-database-postgresql
kubectl wait --for=condition=available --timeout=300s --namespace $GROUP deployment/$INSTANCE_NAME
sleep 3
http --check-status --follow "$INSTANCE_HOST_DEPLOY/$INSTANCE_NAME"
./destroy.sh $GROUP $INSTANCE_NAME-db $INSTANCE_NAME-preset $INSTANCE_NAME
kubectl delete pvc --namespace $GROUP data-$INSTANCE_NAME-db-database-postgresql-0
