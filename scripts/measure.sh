#!/bin/bash

for ((i = 0; i < 2; i++)); do
    # create/deploy "concurrently"
    # (export MYID="ivo-dhis2-${i}" && export ACCESS_TOKEN="" && eval $(./login.sh) && echo $ACCESS_TOKEN && ./dhis2-create.sh $MYID whoami && ./dhis2-deploy.sh $MYID whoami) &

    # create/deploy
    # (export MYID="ivo-dhis2-${i}" && export ACCESS_TOKEN="" && eval $(./login.sh) && echo $ACCESS_TOKEN && ./dhis2-create.sh $MYID whoami && ./dhis2-deploy.sh $MYID whoami)

    # destroy
    (export MYID="ivo-dhis2-${i}" && export ACCESS_TOKEN="" && eval $(./login.sh) && echo $ACCESS_TOKEN && ./destroy.sh $MYID whoami)
done

