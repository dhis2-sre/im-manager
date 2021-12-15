tag ?= latest
version ?= $(shell yq e '.version' helm/Chart.yaml)
clean-cmd = docker compose down --remove-orphans --volumes

binary:
	go build -o im-manager -ldflags "-s -w" ./cmd/serve

smoke-test:
	docker compose up -d database
	sleep 3
	IMAGE_TAG=$(tag) docker compose up -d prod

prod-image:
	IMAGE_TAG=$(tag) docker compose build prod

push-prod:
	IMAGE_TAG=$(tag) docker compose push prod

dev:
	docker compose up --build dev database

cluster-dev:
	skaffold dev

test: clean
	docker compose up -d database
	docker compose run --no-deps test
	$(clean-cmd)

dev-test: clean
	docker compose run --no-deps dev-test
	$(clean-cmd)

clean:
	$(clean-cmd)
	go clean

helm-chart:
	@helm package helm/chart

publish-helm:
	@curl --user "$(CHART_AUTH_USER):$(CHART_AUTH_PASS)" \
        -F "chart=@im-manager-$(version).tgz" \
        https://helm-charts.fitfit.dk/api/charts

swagger-docs:
	swag init --parseDependency --parseDepth 2 -g ./cmd/serve/main.go --output swagger/docs

swagger-client:
	swagger generate client -f swagger/docs/swagger.yaml -t swagger/client

swagger: swagger-docs swagger-client

di:
	wire gen ./internal/di

.PHONY: binary docker-image push-docker-image dev test dev-test helm-chart publish-helm
