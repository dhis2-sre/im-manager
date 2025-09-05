FROM golang:1.25.1-alpine3.21 AS build

# https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/
ARG KUBECTL_VERSION=v1.33.3
ARG KUBECTL_CHECKSUM=2fcf65c64f352742dc253a25a7c95617c2aba79843d1b74e585c69fe4884afb0

# https://github.com/helm/helm/releases
ARG HELM_VERSION=v3.17.4
ARG HELM_CHECKSUM=c91e3d7293849eff3b4dc4ea7994c338bcc92f914864d38b5789bab18a1d775d

# https://github.com/helmfile/helmfile/releases
ARG HELMFILE_VERSION=1.1.3
ARG HELMFILE_CHECKSUM=80733cd836f8b0d5b2271bf7b4a25b86abf80c4e5890b8ff2635d7a529a6df1b

# https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/
ARG AWS_IAM_AUTHENTICATOR_VERSION=0.7.5
ARG AWS_IAM_AUTHENTICATOR_CHECKSUM=aef183b5b92f2cb135107234c7440f43638caa337190190cdd2ad9fd6bc4928e

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
    wget https://github.com/helmfile/helmfile/releases/download/v${HELMFILE_VERSION}/helmfile_${HELMFILE_VERSION}_linux_amd64.tar.gz && \
    echo "${HELMFILE_CHECKSUM}  helmfile_${HELMFILE_VERSION}_linux_amd64.tar.gz" | sha256sum -c - && \
    tar -xvf helmfile_${HELMFILE_VERSION}_linux_amd64.tar.gz && \
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

FROM alpine:3.22
RUN apk --no-cache -U upgrade \
    && apk add --no-cache postgresql16-client
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
