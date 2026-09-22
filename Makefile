tag ?= latest

keys:
	openssl genpkey -algorithm RSA -out ./rsa_private.pem -pkeyopt rsa_keygen_bits:2048

init:
	pip install 'pre-commit==4.6.2'
	pre-commit install --install-hooks --overwrite
	sh scripts/install-test-tools.sh

check:
	pre-commit run --verbose --all-files --show-diff-on-failure

change-log:
	git-chglog -o CHANGELOG.md

smoke-test:
	IMAGE_TAG=$(tag) docker compose --profile prod up --detach prod

docker-image:
	IMAGE_TAG=$(tag) docker compose --profile prod build prod

push-docker-image:
	IMAGE_TAG=$(tag) docker compose push prod

dev:
	docker compose --profile dev up

prod:
	docker compose --profile prod up

# Shared services allow four packages without multiplying their containers.
# Override with TEST_PARALLEL=2 on resource-constrained Docker hosts.
TEST_PARALLEL ?= 4
TEST_FLAGS ?=
TEST_RUNNER = go run ./internal/testenv/cmd

test:
	$(TEST_RUNNER) -- -race -count=1 -p $(TEST_PARALLEL) $(TEST_FLAGS) ./...

test-integration:
	$(TEST_RUNNER) -- -race -count=1 -p $(TEST_PARALLEL) -skip '^TestInstanceHandler$$' $(TEST_FLAGS) ./...

test-e2e:
	$(TEST_RUNNER) -- -race -count=1 -run '^TestInstanceHandler$$' $(TEST_FLAGS) ./pkg/instance

test-coverage:
	$(TEST_RUNNER) -- -count=1 -p $(TEST_PARALLEL) -coverprofile=./coverage.out ./... && go tool cover -html=./coverage.out -o ./coverage.html

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

.PHONY: test-integration test-e2e
