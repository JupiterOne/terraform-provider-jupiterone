TEST?=$$(go list ./...)
PKG_NAME=jupiterone
DIR=~/.terraform.d/plugins

JUPITERONE_ACCOUNT_ID?=fake
JUPITERONE_API_KEY?=fake
JUPITERONE_REGION?=fake

export JUPITERONE_ACCOUNT_ID JUPITERONE_API_KEY JUPITERONE_REGION

default: build

build: fmtcheck
	go build ./...

install: fmtcheck
	go install .

test:
	go test $(TEST) -v $(TESTARGS) -timeout 120m

testacc: fmtcheck
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m

cassettes: fmtcheck
	rm -f jupiterone/cassettes/*.yaml
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m

fmt:
	@echo "==> Fixing source code with gofmt..."
	gofmt -w -s .

fmtcheck:
	@./scripts/fmtcheck.sh

lint:
	@echo "==> Checking source code against linters..."
	@golangci-lint run ./$(PKG_NAME)/...

tools:
	@echo "==> installing required tooling..."
	go install github.com/client9/misspell/cmd/misspell
	go install github.com/golangci/golangci-lint/cmd/golangci-lint

test-compile:
	@if [ "$(TEST)" = "./..." ]; then \
		echo "ERROR: Set TEST to a specific package. For example,"; \
		echo "  make test-compile TEST=./$(PKG_NAME)"; \
		exit 1; \
	fi
	go test -c $(TEST) $(TESTARGS)

docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

jupiterone/internal/client/schema.graphql:
	@scripts/get_current_schema.bash

jupiterone/internal/client/generated.go: jupiterone/internal/client/*.graphql jupiterone/internal/client/genqlient.yaml
	@go run github.com/Khan/genqlient ./jupiterone/internal/client/genqlient.yaml

generate-client: jupiterone/internal/client/generated.go

.PHONY: build test testacc cassettes fmtcheck lint tools test-compile docs
