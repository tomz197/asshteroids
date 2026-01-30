APP_NAME=game
BIN_DIR=bin

.PHONY: build run clean fmt

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/game

run:
	go run ./cmd/game

fmt:
	go fmt ./...

clean:
	rm -rf $(BIN_DIR)
