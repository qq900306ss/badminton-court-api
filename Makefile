build-lambda:
	GOOS=linux GOARCH=arm64 go build -o bootstrap ./cmd/lambda
	zip function.zip bootstrap
	rm bootstrap

build-local:
	go build ./cmd/server

run:
	go run ./cmd/server

fmt:
	gofmt -w .

.PHONY: build-lambda build-local run fmt
