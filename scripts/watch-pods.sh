#!/bin/sh

kubectl -n instance-manager-feature get pods \
  --selector app.kubernetes.io/instance=im-manager-sigterm --watch
