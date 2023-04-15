tag ?= latest
clean-cmd = docker compose down --remove-orphans --volumes

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
	docker compose up -d database rabbitmq jwks
	sleep 10
	IMAGE_TAG=$(tag) docker compose up -d prod

docker-image:
	IMAGE_TAG=$(tag) docker compose build prod

push-docker-image:
	IMAGE_TAG=$(tag) docker compose push prod

dev:
	docker compose up --build dev database rabbitmq redis jwks

test: clean
	docker compose up -d database rabbitmq jwks
	docker compose run --no-deps test
	$(clean-cmd)

test-coverage: clean
	docker compose up -d database rabbitmq jwks
	docker compose run --no-deps test-coverage
	$(clean-cmd)

clean:
	$(clean-cmd)
	go clean

swagger-clean:
	rm -f swagger/swagger.yaml

swagger-spec:
	swagger generate spec -o swagger/swagger.yaml --scan-models
	swagger validate swagger/swagger.yaml

swagger: swagger-clean swagger-spec

.PHONY: init check docker-image push-docker-image dev test
