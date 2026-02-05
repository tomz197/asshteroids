# ASSHTEROIDS

A multiplayer terminal-based Asteroids game playable over SSH.

## Features

- Classic Asteroids gameplay in your terminal
- Multiplayer over SSH - multiple players share the same game world
- Web landing page with connection instructions
- Docker support for easy deployment

## Controls

| Action       | Keys                          |
|--------------|-------------------------------|
| Move Up      | `W` / `I` / `↑`               |
| Move Down    | `S` / `K` / `↓`               |
| Move Left    | `A` / `J` / `←`               |
| Move Right   | `D` / `L` / `→`               |
| Shoot        | `Space`                       |
| Quit         | `Q`                           |

## Quick Start

### Run Locally

Play the game directly in your terminal:

```sh
make run
```

### Run with Docker Compose (Recommended)

Start both the SSH server (port 22) and web landing page (port 8080):

```sh
docker compose up -d --build
```

Connect and play:

```sh
ssh -t localhost
```

View the landing page at http://localhost:8080

Stop the services:

```sh
docker compose down
```

## Running Without Docker

### SSH Server

Generate a host key and start the SSH server:

```sh
make generate-host-key
SSH_HOST_KEY=./keys/host_key make run-ssh
```

Connect:

```sh
ssh -t localhost
```

### Web Landing Page

```sh
SSH_DISPLAY_HOST=your-server.com make run-web
```

## Building

```sh
# Build all binaries
make build        # Local game
make build-ssh    # SSH server
make build-web    # Web server

# Run built binaries
./bin/game
./bin/asteroids-ssh
./bin/asteroids-web
```

## Environment Variables

### SSH Server

| Variable       | Default   | Description                    |
|----------------|-----------|--------------------------------|
| `SSH_HOST`     | `0.0.0.0` | Host to bind the SSH server    |
| `SSH_PORT`     | `22`      | Port for the SSH server        |
| `SSH_HOST_KEY` | -         | Path to SSH host key file      |

### Web Server

| Variable           | Default           | Description                              |
|--------------------|-------------------|------------------------------------------|
| `WEB_HOST`         | `0.0.0.0`         | Host to bind the web server              |
| `WEB_PORT`         | `8080`            | Port for the web server                  |
| `SSH_DISPLAY_HOST` | `your-server.com` | SSH host shown on the landing page       |

## Make Targets

| Target              | Description                              |
|---------------------|------------------------------------------|
| `run`               | Run the local game                       |
| `run-ssh`           | Run the SSH server                       |
| `run-web`           | Run the web landing page                 |
| `build`             | Build the local game binary              |
| `build-ssh`         | Build the SSH server binary              |
| `build-web`         | Build the web server binary              |
| `generate-host-key` | Generate SSH host key for local testing  |
| `docker-build`      | Build Docker image                       |
| `docker-run`        | Run Docker container                     |
| `docker-stop`       | Stop Docker container                    |
| `docker-logs`       | View Docker container logs               |
| `fmt`               | Format Go code                           |
| `clean`             | Remove build artifacts                   |
