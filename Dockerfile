FROM golang:1.22.2-alpine3.18 AS build

# https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/
ARG KUBECTL_VERSION=v1.28.0
ARG KUBECTL_CHECKSUM=4717660fd1466ec72d59000bb1d9f5cdc91fac31d491043ca62b34398e0799ce

# https://github.com/helm/helm/releases
ARG HELM_VERSION=v3.12.3
ARG HELM_CHECKSUM=1b2313cd198d45eab00cc37c38f6b1ca0a948ba279c29e322bdf426d406129b5

# https://github.com/roboll/helmfile/releases
ARG HELMFILE_VERSION=v0.144.0
ARG HELMFILE_CHECKSUM=dcf865a715028d3a61e2fec09f2a0beaeb7ff10cde32e096bf94aeb9a6eb4f02

# https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/
ARG AWS_IAM_AUTHENTICATOR_VERSION=0.6.11
ARG AWS_IAM_AUTHENTICATOR_CHECKSUM=8593d0c5125f8fba4589008116adf12519cdafa56e1bfa6b11a277e2886fc3c8

# https://github.com/cespare/reflex/releases
ARG REFLEX_VERSION=v0.3.1

RUN apk add gcc musl-dev git && \
\
    wget https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    echo "${KUBECTL_CHECKSUM}  kubectl" | sha256sum -c - && \
    install -o root -g root -m 0755 kubectl /usr/bin/kubectl && \
\
    wget https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz && \
    echo "${HELM_CHECKSUM}  helm-${HELM_VERSION}-linux-amd64.tar.gz" | sha256sum -c - && \
    tar -xvf helm-${HELM_VERSION}-linux-amd64.tar.gz && \
    install -o root -g root -m 0755 linux-amd64/helm /usr/bin/helm && \
\
    wget -O helmfile https://github.com/roboll/helmfile/releases/download/${HELMFILE_VERSION}/helmfile_linux_amd64 && \
    echo "${HELMFILE_CHECKSUM}  helmfile" | sha256sum -c - && \
    install -o root -g root -m 0755 helmfile /usr/bin/helmfile && \
\
    wget -O aws-iam-authenticator https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/v${AWS_IAM_AUTHENTICATOR_VERSION}/aws-iam-authenticator_${AWS_IAM_AUTHENTICATOR_VERSION}_linux_amd64 && \
    echo "${AWS_IAM_AUTHENTICATOR_CHECKSUM}  aws-iam-authenticator" | sha256sum -c - && \
    install -o root -g root -m 0755 aws-iam-authenticator /usr/bin/aws-iam-authenticator

WORKDIR /src
RUN go install github.com/cespare/reflex@${REFLEX_VERSION}
COPY go.mod go.sum ./
RUN go mod download -x
COPY . .
RUN go build -o /app/im-manager -ldflags "-s -w" ./cmd/serve

FROM alpine:3.18
RUN apk --no-cache -U upgrade \
    && apk add --no-cache postgresql-client
COPY --from=build /usr/bin/kubectl /usr/bin/kubectl
COPY --from=build /usr/bin/helm /usr/bin/helm
COPY --from=build /usr/bin/helmfile /usr/bin/helmfile
COPY --from=build /usr/bin/aws-iam-authenticator /usr/bin/aws-iam-authenticator
WORKDIR /app
COPY --from=build /app/im-manager .
COPY --from=build /src/swagger/swagger.yaml ./swagger/
# helmfile invokes helm in the folder which contains the helmfile.yaml and requires write access to .config/ and .cache/ in the same folder
COPY --from=build --chown=guest:users /src/stacks ./stacks
USER guest
ENTRYPOINT ["/app/im-manager"]
