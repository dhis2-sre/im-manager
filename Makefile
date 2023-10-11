tag ?= latest

keys:
	openssl genpkey -algorithm RSA -out ./rsa_private.pem -pkeyopt rsa_keygen_bits:2048
	openssl rsa -in ./rsa_private.pem -pubout -out ./rsa_public.pem

init:
	pip install pre-commit
	pre-commit install --install-hooks --overwrite

	go install github.com/direnv/direnv@latest
	direnv version

	go install golang.org/x/tools/cmd/goimports@latest

	go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec --version

	go install github.com/go-swagger/go-swagger/cmd/swagger@latest
	swagger version

check:
	pre-commit run --all-files --show-diff-on-failure

smoke-test:
	docker compose up -d database rabbitmq
	sleep 10
	IMAGE_TAG=$(tag) docker compose up -d prod

docker-image:
	IMAGE_TAG=$(tag) docker compose build prod

push-docker-image:
	IMAGE_TAG=$(tag) docker compose push prod

dev:
	docker compose --profile dev up

test:
	go test -race ./...

test-coverage:
	go test -coverprofile=./coverage.out ./... && go tool cover -html=./coverage.out -o ./coverage.html

clean:
	docker compose --profile dev down --remove-orphans --volumes
	go clean

swagger-clean:
	rm -f swagger/swagger.yaml

swagger-spec:
	swagger generate spec -o swagger/swagger.yaml
	swagger validate swagger/swagger.yaml

swagger: swagger-clean swagger-spec

.PHONY: keys init check smoke-test docker-image push-docker-image dev cluster-dev test test-coverage clean-dev clean-cluster-dev swagger-clean swagger-spec swagger
