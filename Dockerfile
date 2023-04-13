FROM golang:1.20.1-alpine3.17 AS build

ARG KUBECTL_VERSION=v1.23.5
ARG KUBECTL_CHECKSUM=715da05c56aa4f8df09cb1f9d96a2aa2c33a1232f6fd195e3ffce6e98a50a879

ARG HELM_VERSION=v3.8.1
ARG HELM_CHECKSUM=d643f48fe28eeb47ff68a1a7a26fc5142f348d02c8bc38d699674016716f61cd

ARG HELMFILE_VERSION=v0.143.3
ARG HELMFILE_CHECKSUM=a30a5c9f64c8eba2123625497913f9ad210a047e997f2363cda3189cbac8f970

ARG AWS_IAM_AUTHENTICATOR_VERSION=1.21.2/2021-07-05
ARG AWS_IAM_AUTHENTICATOR_CHECKSUM=fe958eff955bea1499015b45dc53392a33f737630efd841cd574559cc0f41800

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
    wget -O aws-iam-authenticator https://amazon-eks.s3.us-west-2.amazonaws.com/${AWS_IAM_AUTHENTICATOR_VERSION}/bin/linux/amd64/aws-iam-authenticator && \
    echo "${AWS_IAM_AUTHENTICATOR_CHECKSUM}  aws-iam-authenticator" | sha256sum -c - && \
    install -o root -g root -m 0755 aws-iam-authenticator /usr/bin/aws-iam-authenticator

WORKDIR /src
RUN go install github.com/cespare/reflex@${REFLEX_VERSION}
COPY go.mod go.sum ./
RUN go mod download -x
COPY . .
RUN go build -o /app/im-manager -ldflags "-s -w" ./cmd/serve

FROM alpine:3.17
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
