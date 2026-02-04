APP_NAME=game
SSH_NAME=asteroids-ssh
BIN_DIR=bin
DOCKER_IMAGE=asteroids-ssh
DOCKER_TAG=latest

.PHONY: build build-ssh run run-ssh clean fmt docker-build docker-run docker-stop

# Local builds
build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/game

build-ssh:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(SSH_NAME) ./cmd/ssh

# Local run
run:
	go run ./cmd/game

run-ssh:
	go run ./cmd/ssh

fmt:
	go fmt ./...

clean:
	rm -rf $(BIN_DIR)

# Docker targets
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-run:
	docker run -d --name asteroids-ssh \
		-p 22:22 \
		-v asteroids-keys:/app/keys \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

docker-stop:
	docker stop asteroids-ssh || true
	docker rm asteroids-ssh || true

docker-logs:
	docker logs -f asteroids-ssh

# Generate SSH host key (for local testing without Docker)
generate-host-key:
	mkdir -p keys
	ssh-keygen -t ed25519 -f keys/host_key -N ""
