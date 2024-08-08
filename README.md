#foo
# TODO

* Write readme

# Add a group
* Add group in IM (either through the UI or by using the user script found [here](scripts/users/createGroup.sh)
* Update [values file](https://github.com/dhis2-sre/im-manager/blob/8cb9a5959e334b835188fa07e801996ff2410b7c/helm/chart/values.yaml#L12) or for an individual environment such as [prod](https://github.com/dhis2-sre/im-manager/blob/8cb9a5959e334b835188fa07e801996ff2410b7c/helm/data/values/prod/values.yaml#L1)
* Update the profiles section of the [skaffold file](https://github.com/dhis2-sre/im-manager/blob/8cb9a5959e334b835188fa07e801996ff2410b7c/skaffold.yaml#L96) to include the group
* Update backup schedule to include the group for either [dev](https://github.com/dhis2-sre/dhis2-infrastructure/blob/b9f53752ca9cb16883f2f78cae5fca42b4087b1f/modules/k8s/helm-backup-dev.tf#L1) or [prod](https://github.com/dhis2-sre/dhis2-infrastructure/blob/b9f53752ca9cb16883f2f78cae5fca42b4087b1f/modules/k8s/helm-backup-prod.tf#L1)
