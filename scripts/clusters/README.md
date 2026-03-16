# Adding a new cluster

Note that we need to encrypt the kubeconfig file when adding it with the `create.sh` script and the `.sops.yaml` file in the current directory defaults to the `im-nonprod-secrets` KMS key, which is to be used only for dev/feature environments. When adding a new cluster on prod the key needs to be the `im-prod-secrets` one.
