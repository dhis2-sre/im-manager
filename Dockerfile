FROM golang:1.25.5-alpine3.21 AS build

ARG TARGETARCH

# https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/
ARG KUBECTL_VERSION=v1.33.3
ARG KUBECTL_CHECKSUM_AMD64=2fcf65c64f352742dc253a25a7c95617c2aba79843d1b74e585c69fe4884afb0
ARG KUBECTL_CHECKSUM_ARM64=3d514dbae5dc8c09f773df0ef0f5d449dfad05b3aca5c96b13565f886df345fd

# https://github.com/helm/helm/releases
ARG HELM_VERSION=v3.17.4
ARG HELM_CHECKSUM_AMD64=c91e3d7293849eff3b4dc4ea7994c338bcc92f914864d38b5789bab18a1d775d
ARG HELM_CHECKSUM_ARM64=460a31d1511abb5ad776a26a2a3f0f1382a241b2df3c6d725b0f63c9058ba15a

# https://github.com/helmfile/helmfile/releases
ARG HELMFILE_VERSION=1.2.3
ARG HELMFILE_CHECKSUM_AMD64=e238185ff094ecf41d81f2938711569803698355c7fc6081fa047bfa27b4c55c
ARG HELMFILE_CHECKSUM_ARM64=16baf067534d4176b75cac3f3b791f5954ecba8f353ba7d7cbe0c1dd7396f819

# https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/
ARG AWS_IAM_AUTHENTICATOR_VERSION=0.7.5
ARG AWS_IAM_AUTHENTICATOR_CHECKSUM_AMD64=aef183b5b92f2cb135107234c7440f43638caa337190190cdd2ad9fd6bc4928e
ARG AWS_IAM_AUTHENTICATOR_CHECKSUM_ARM64=80ab2b0d5a139e55408c785703e8fef8f4974a73409046b3e13df19b2a780a51

# https://github.com/cespare/reflex/releases
ARG REFLEX_VERSION=v0.3.1

RUN apk add gcc musl-dev git && \
\
    case "$TARGETARCH" in \
        amd64) KUBECTL_CHECKSUM="$KUBECTL_CHECKSUM_AMD64"; HELM_CHECKSUM="$HELM_CHECKSUM_AMD64"; HELMFILE_CHECKSUM="$HELMFILE_CHECKSUM_AMD64"; AWS_IAM_AUTHENTICATOR_CHECKSUM="$AWS_IAM_AUTHENTICATOR_CHECKSUM_AMD64" ;; \
        arm64) KUBECTL_CHECKSUM="$KUBECTL_CHECKSUM_ARM64"; HELM_CHECKSUM="$HELM_CHECKSUM_ARM64"; HELMFILE_CHECKSUM="$HELMFILE_CHECKSUM_ARM64"; AWS_IAM_AUTHENTICATOR_CHECKSUM="$AWS_IAM_AUTHENTICATOR_CHECKSUM_ARM64" ;; \
    esac && \
\
    wget https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl && \
    echo "${KUBECTL_CHECKSUM}  kubectl" | sha256sum -c - && \
    install -o root -g root -m 0755 kubectl /usr/bin/kubectl && \
\
    wget https://get.helm.sh/helm-${HELM_VERSION}-linux-${TARGETARCH}.tar.gz && \
    echo "${HELM_CHECKSUM}  helm-${HELM_VERSION}-linux-${TARGETARCH}.tar.gz" | sha256sum -c - && \
    tar -xvf helm-${HELM_VERSION}-linux-${TARGETARCH}.tar.gz && \
    install -o root -g root -m 0755 linux-${TARGETARCH}/helm /usr/bin/helm && \
\
    wget https://github.com/helmfile/helmfile/releases/download/v${HELMFILE_VERSION}/helmfile_${HELMFILE_VERSION}_linux_${TARGETARCH}.tar.gz && \
    echo "${HELMFILE_CHECKSUM}  helmfile_${HELMFILE_VERSION}_linux_${TARGETARCH}.tar.gz" | sha256sum -c - && \
    tar -xvf helmfile_${HELMFILE_VERSION}_linux_${TARGETARCH}.tar.gz && \
    install -o root -g root -m 0755 helmfile /usr/bin/helmfile && \
\
    wget -O aws-iam-authenticator https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/v${AWS_IAM_AUTHENTICATOR_VERSION}/aws-iam-authenticator_${AWS_IAM_AUTHENTICATOR_VERSION}_linux_${TARGETARCH} && \
    echo "${AWS_IAM_AUTHENTICATOR_CHECKSUM}  aws-iam-authenticator" | sha256sum -c - && \
    install -o root -g root -m 0755 aws-iam-authenticator /usr/bin/aws-iam-authenticator

WORKDIR /src
RUN go install github.com/cespare/reflex@${REFLEX_VERSION}
COPY go.mod go.sum ./
RUN go mod download -x
COPY . .
RUN CGO_ENABLED=0 go build -o /app/im-manager -ldflags "-s -w" ./cmd/serve

FROM alpine:3.23
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
