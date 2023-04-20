# Prerequisites

It's highly recommended deploying this application on a Kubernetes cluster.

A small project which can assists with the creation of a cluster can be found [here](https://github.com/dhis2-sre/im-cluster).

# Requirements

The following applications are needed by the scripts

* [httpie](https://github.com/httpie/httpie)
* [jq](https://github.com/stedolan/jq)

# Quick Start

```sh
./signUp.sh
export ACCESS_TOKEN="" && eval $(./signIn.sh) && echo $ACCESS_TOKEN
USER_ID=$(./me.sh | jq -r '.ID')
export ACCESS_TOKEN="" && eval $(./signInAdmin.sh) && echo $ACCESS_TOKEN
./addUserToGroup.sh $USER_ID whoami
export ACCESS_TOKEN="" && eval $(./signIn.sh) && echo $ACCESS_TOKEN
./me.sh
```

# Getting started

This service can be started locally or in a cluster.

Running `make dev` will start the service along with its dependencies locally.

Run the below command to confirm the service is running.

```sh
http :8080/health
```

## Docs

Once the application is up and running its documentation can be found
at [http://localhost:8080/docs](http://localhost:8080/docs).

## Environment

For the sake of simplicity the user scripts relies on a few environment variables. So in order to interact with the
application the environment needs to be configured.

An example of such configuration can be found in `.env.example`. It's recommended that you make a copy and populate it
with your own credentials.

```sh
cp .env.example .env
```

In order to automatically export the variables, the author of this application recommends [direnv](https://direnv.net/).

If we're dealing with a locally running instance, the variable `IM_HOST`, should be defined as below.

```sh
IM_HOST=:8080
```

The environment is configured correctly if the `health.sh` script returns 200 and "status: up"

## Signup

A user can be created using the `signUp.sh` script.

The script will automatically use the credentials defined in `.env`.

```sh
./signUp.sh
```

## Signin

After successfully signing up the newly created user can be used to sign in and retrieve an access token.

```sh
./signIn.sh
```

The above script echos the command used to export the access token as the variable ACCESS_TOKEN.

So the below command can be used as a shortcut to signin and export the access token.

```sh
export ACCESS_TOKEN && eval $(./signIn.sh) && echo $ACCESS_TOKEN
```

Assuming the signin was successful the access token will be printed on the terminal.

## Me

The details of the current user can be retrieved by running the `me.sh` script.

```sh
./me.sh
```

## Groups

For the user to actually be able to do anything it needs to be part of a group. Only administrative users can create and
add other users to groups.

The credentials for the initially created administrator can be found in `helm/data/secrets/*/values.yaml`.

### Create group

A group can be created using the `createGroup.sh` script.

Run the below command to create a group called "test-group" with hostname "im.c.127.0.0.1.nip.io"

```sh
./createGroup.sh test-group im.c.127.0.0.1.nip.io
```

### Cluster configuration

WIP

### Add user to group

A user can be added to a given group using the `addUserToGroup.sh` script.

Run the below command to add the user with id "123" to the group called "test-group"

```sh
./addUserToGroup.sh 123 test-group
```

# General

All scripts rely on the environment variable $ACCESS_TOKEN being exported.

Valid access and refresh tokens can be retrieved by running `signIn.sh` (or `signInAdmin.sh`) and exported
with `eval $(./signIn.sh)` (or `eval $(./signInAdmin.sh)`).

The following scripts require Admin access:

* [createGroup.sh](createGroup.sh)
* [addUserToGroup.sh](addUserToGroup.sh)
* [addClusterConfigToGroup.sh](addClusterConfigToGroup.sh)
