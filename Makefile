DATABASE_URL ?= postgres://grpc:grpc@localhost:5432/go_foundation?sslmode=disable

.PHONY: tools proto fmt test run keys docker-up docker-down logs migrate-up migrate-down migrate-create tidy

tools:
	go install github.com/bufbuild/buf/cmd/buf@v1.47.2
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.1
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.23.0
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.23.0
	go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

proto:
	buf generate

fmt:
	gofmt -w ./cmd ./internal

test: proto
	go test -race ./...

run: proto
	go run ./cmd/server

keys:
	./scripts/generate-dev-keys.sh

docker-up: keys
	docker compose up --build -d

docker-down:
	docker compose down

logs:
	docker compose logs -f api

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-create:
	@test -n "$(name)" || (echo "usage: make migrate-create name=create_something" && exit 1)
	migrate create -ext sql -dir migrations -seq $(name)

tidy:
	go mod tidy
