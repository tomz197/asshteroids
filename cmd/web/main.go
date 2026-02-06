package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/tomz197/asteroids/internal/config"
)

const (
	defaultHost = "0.0.0.0"
	defaultPort = "8080"
)

//go:embed index.html
var htmlPage string

func main() {
	host := config.GetEnv("WEB_HOST", defaultHost)
	port := config.GetEnv("WEB_PORT", defaultPort)
	sshHost := config.GetEnv("SSH_DISPLAY_HOST", "your-server.com")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		page := strings.Replace(htmlPage, "{{.SSHHost}}", sshHost, -1)
		fmt.Fprint(w, page)
	})

	addr := fmt.Sprintf("%s:%s", host, port)
	log.Printf("Starting web server on http://%s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
