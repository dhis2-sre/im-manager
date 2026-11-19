# User scripts

Please see https://github.com/dhis2-sre/im-doc/tree/master/api#getting-started for an in-depth introduction to the user scripts accompanying each service

# Examples

## Deploy DHIS2
```sh
./deploy-dhis2.sh my-group my-instance
```

## Find a deployment
```sh
./findByName.sh my-group my-instance
```

## Destroy all deployments starting with "my-"
```sh
./destroy-deployment.sh $(./findDeployments.sh | jq -r '.[] | select(.name=="my-group") | .deployments[] | select(.name | startswith("my-")) | .id')
```
