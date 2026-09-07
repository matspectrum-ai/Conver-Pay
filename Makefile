SHELL := /bin/bash
API_DIR := apps/api
GOOSE_VERSION := v3.27.3

.PHONY: api-run api-test api-vet api-fmt api-fmt-check ci db-up db-down

api-run:
	cd $(API_DIR) && go run ./cmd/api

api-test:
	cd $(API_DIR) && go test ./...

api-vet:
	cd $(API_DIR) && go vet ./...

api-fmt:
	gofmt -w $$(find $(API_DIR) -name '*.go' -type f)

api-fmt-check:
	@test -z "$$(gofmt -l $$(find $(API_DIR) -name '*.go' -type f))" || \
		(echo "Go files need formatting:"; gofmt -l $$(find $(API_DIR) -name '*.go' -type f); exit 1)

ci: api-fmt-check api-vet api-test

db-up:
	@test -n "$$DATABASE_URL" || (echo "DATABASE_URL is required"; exit 1)
	cd $(API_DIR) && go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir ../../migrations postgres "$$DATABASE_URL" up

db-down:
	@test -n "$$DATABASE_URL" || (echo "DATABASE_URL is required"; exit 1)
	cd $(API_DIR) && go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir ../../migrations postgres "$$DATABASE_URL" down
