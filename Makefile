.PHONY: compile-templates
compile-templates:
	cd deploy/generators && npm i && npm run compile-templates

.PHONY: start
start:
	go run cmd/server.go

.PHONY: install
install:
	go mod download

.PHONY: test
test:
	go test -short -count=1 ./...

.PHONY: lint
lint:
	sh bin/lint-imports.sh

.PHONY: schemas
schemas:
	sh bin/gen-resources.sh

.PHONY: tools
tools: install-codegen

install-codegen:
	go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@latest
