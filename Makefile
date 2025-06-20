tag ?= latest

keys:
	openssl genpkey -algorithm RSA -out ./rsa_private.pem -pkeyopt rsa_keygen_bits:2048

init:
	pip install pre-commit
	pre-commit clean
	pre-commit install --install-hooks --overwrite

	go install github.com/direnv/direnv@latest
	direnv version

	go install golang.org/x/tools/cmd/goimports@latest

	go install github.com/go-swagger/go-swagger/cmd/swagger@latest
	swagger version

	go install github.com/git-chglog/git-chglog/cmd/git-chglog@latest

check:
	pre-commit run --verbose --all-files --show-diff-on-failure

change-log:
	git-chglog -o CHANGELOG.md

smoke-test:
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

clean-dev:
	docker compose --profile dev down --remove-orphans --volumes
	go clean

clean:
	docker compose --profile prod down --remove-orphans --volumes

swagger-clean:
	rm -f swagger/swagger.yaml

swagger-spec:
	swagger generate spec -o swagger/swagger.yaml
	swagger validate swagger/swagger.yaml

swagger: swagger-clean swagger-spec

.PHONY: keys init check smoke-test docker-image push-docker-image dev cluster-dev test test-coverage clean-dev clean-cluster-dev swagger-clean swagger-spec swagger
