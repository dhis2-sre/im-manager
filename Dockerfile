FROM golang:1.17-alpine AS build
RUN apk add gcc musl-dev git && \
\
    wget https://dl.k8s.io/release/v1.21.0/bin/linux/amd64/kubectl && \
    echo "9f74f2fa7ee32ad07e17211725992248470310ca1988214518806b39b1dad9f0  kubectl" | sha256sum -c - && \
    install -o root -g root -m 0755 kubectl /usr/bin/kubectl && \
\
    wget https://get.helm.sh/helm-v3.6.0-linux-amd64.tar.gz && \
    echo "0a9c80b0f211791d6a9d36022abd0d6fd125139abe6d1dcf4c5bf3bc9dcec9c8  helm-v3.6.0-linux-amd64.tar.gz" | sha256sum -c - && \
    tar -xvf helm-v3.6.0-linux-amd64.tar.gz && \
    install -o root -g root -m 0755 linux-amd64/helm /usr/bin/helm && \
\
    wget -O helmfile https://github.com/roboll/helmfile/releases/download/v0.139.8/helmfile_linux_amd64 && \
    echo "674efc68fb0771cde17389435c7c37672270efe84d06f74afff977b98ac43b84  helmfile" | sha256sum -c - && \
    install -o root -g root -m 0755 helmfile /usr/bin/helmfile && \
\
    wget -O aws-iam-authenticator https://amazon-eks.s3.us-west-2.amazonaws.com/1.19.6/2021-01-05/bin/linux/amd64/aws-iam-authenticator && \
    echo "fe958eff955bea1499015b45dc53392a33f737630efd841cd574559cc0f41800  aws-iam-authenticator" | sha256sum -c - && \
    install -o root -g root -m 0755 aws-iam-authenticator /usr/bin/aws-iam-authenticator

WORKDIR /src
RUN go get github.com/cespare/reflex
COPY go.mod go.sum ./
RUN go mod download -x
COPY . .
RUN go build -o /app/im-manager -ldflags "-s -w" ./cmd/serve

FROM alpine:3.14
RUN apk --no-cache -U upgrade
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
CMD ["/app/im-manager"]
