# Asteroids

Small terminal-based Asteroids game in Go.

## Run (local)

```sh
make run
```

## Build (local)

```sh
make build
./bin/game
```

## Run over SSH (Docker Compose)

Start the SSH server container (listens on port `22`):

```sh
docker compose up -d --build
```

Connect and play (PTY allocation is required):

```sh
ssh -t localhost
```

Stop:

```sh
docker compose down
```

## Run over SSH (no Docker)

Generate a host key, then run the SSH server:

```sh
make generate-host-key
SSH_HOST_KEY=./keys/host_key make run-ssh
```

Connect:

```sh
ssh -t localhost
```
