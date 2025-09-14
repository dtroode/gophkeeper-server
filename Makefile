SHELL := /bin/sh

.PHONY: mocks test cover test-integration

mocks:
	docker run --rm -e GOTOOLCHAIN=auto -v "$$PWD":/src -w /src vektra/mockery

test:
	go test ./... -count=1 -cover

cover:
	go test ./... -count=1 -coverprofile=profiles.coverage.out && \
		go tool cover -func=profiles.coverage.out | tail -n 1

test-integration:
	go test -tags=integration ./internal/repository/postgres -v -count=1

