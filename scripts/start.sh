#!/bin/sh

# Start web server in background
/app/asteroids-web &

# Start SSH server in foreground
exec /app/asteroids-ssh
