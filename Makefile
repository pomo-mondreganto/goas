.PHONY: lint
lint:
	golangci-lint run -v

.PHONY: test
test:
	go test -race ./...

.PHONY: start
start:
	docker-compose up --build -d
