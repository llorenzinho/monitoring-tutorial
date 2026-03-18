// traffic-gen generates continuous HTTP traffic against the trace-app backend
// to produce visible data in Grafana (metrics, logs, traces).
//
// Usage:
//
//	go run scripts/traffic-gen.go
//	go run scripts/traffic-gen.go -base-url http://trace-app:8800 -interval 500ms
//
// Stop with Ctrl+C.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	baseURL  = flag.String("base-url", "http://trace-app:8800", "Base URL of the backend service")
	interval = flag.Duration("interval", 300*time.Millisecond, "Delay between requests")
)

func main() {
	flag.Parse()

	// Resolve trace-app to 127.0.0.1 without touching /etc/hosts.
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				if host == "trace-app" {
					addr = net.JoinHostPort("127.0.0.1", port)
				}
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	log.Printf("Starting traffic generator → %s  (interval: %s)", *baseURL, *interval)
	log.Println("Press Ctrl+C to stop.")

	var (
		total   int
		success int
		errors  int
	)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			fmt.Printf("\nDone. total=%d  success=%d  errors=%d\n", total, success, errors)
			return
		case <-ticker.C:
			var err error
			switch rand.Intn(5) {
			case 0:
				err = get(client, *baseURL+"/")
			case 1:
				err = get(client, *baseURL+"/items")
			case 2:
				id := rand.Intn(10) + 1
				err = get(client, fmt.Sprintf("%s/items/%d", *baseURL, id))
			case 3:
				err = createItem(client, *baseURL+"/items")
			case 4:
				id := rand.Intn(10) + 1
				err = deleteItem(client, fmt.Sprintf("%s/items/%d", *baseURL, id))
			}
			total++
			if err != nil {
				log.Printf("ERROR  %v", err)
				errors++
			} else {
				success++
			}
			if total%50 == 0 {
				log.Printf("stats  total=%d  success=%d  errors=%d", total, success, errors)
			}
		}
	}
}

func get(c *http.Client, url string) error {
	resp, err := c.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	log.Printf("GET    %-40s  %d", url, resp.StatusCode)
	return nil
}

func createItem(c *http.Client, url string) error {
	names := []string{"apple", "banana", "cherry", "date", "elderberry", "fig", "grape"}
	payload := map[string]string{
		"name":        names[rand.Intn(len(names))],
		"description": fmt.Sprintf("generated at %s", time.Now().Format(time.RFC3339)),
	}
	body, _ := json.Marshal(payload)
	resp, err := c.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	log.Printf("POST   %-40s  %d", url, resp.StatusCode)
	return nil
}

func deleteItem(c *http.Client, url string) error {
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("DELETE %s: %w", url, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	log.Printf("DELETE %-40s  %d", url, resp.StatusCode)
	return nil
}
