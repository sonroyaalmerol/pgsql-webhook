package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/lib/pq"
)

type Config struct {
	DatabaseURL string
	WebhookURL  string
	Channel     string
}

type Event struct {
	Operation string          `json:"operation"`
	Timestamp string          `json:"timestamp"`
	Table     string          `json:"table"`
	Data      json.RawMessage `json:"data"`
	OldData   json.RawMessage `json:"old_data,omitempty"`
}

func main() {
	// Build database URL from individual params or use DATABASE_URL directly
	databaseURL := getDatabaseURL()

	config := Config{
		DatabaseURL: databaseURL,
		WebhookURL:  getEnv("WEBHOOK_URL", "http://localhost:1880/authentik-webhook"),
		Channel:     getEnv("CHANNEL", "authentik_changes"),
	}

	log.Printf("Starting pgsql-webhook")
	log.Printf("Webhook URL: %s", config.WebhookURL)
	log.Printf("Channel: %s", config.Channel)

	for {
		if err := listen(config); err != nil {
			log.Printf("Error: %v. Reconnecting in 5 seconds...", err)
			time.Sleep(5 * time.Second)
		}
	}
}

func getDatabaseURL() string {
	// If DATABASE_URL is set, use it directly
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		return dbURL
	}

	// Otherwise, build from individual components with proper URL encoding
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "password")
	dbname := getEnv("DB_NAME", "postgres")
	sslmode := getEnv("DB_SSLMODE", "disable")

	// URL encode the username and password to handle special characters
	encodedUser := url.QueryEscape(user)
	encodedPassword := url.QueryEscape(password)

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		encodedUser, encodedPassword, host, port, dbname, sslmode,
	)
}

func listen(config Config) error {
	db, err := sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping: %w", err)
	}

	log.Println("Connected to PostgreSQL")

	connStr := config.DatabaseURL
	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Printf("Listener error: %v", err)
		}
	}

	listener := pq.NewListener(connStr, 10*time.Second, time.Minute, reportProblem)
	err = listener.Listen(config.Channel)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	defer listener.Close()

	log.Printf("Listening on channel: %s", config.Channel)
	log.Println("Waiting for notifications...")

	for {
		select {
		case notification := <-listener.Notify:
			if notification == nil {
				continue
			}

			log.Printf("Received notification: %s", notification.Extra)

			var event Event
			if err := json.Unmarshal([]byte(notification.Extra), &event); err != nil {
				log.Printf("Failed to parse notification: %v", err)
				continue
			}

			if err := sendWebhook(config.WebhookURL, event); err != nil {
				log.Printf("Failed to send webhook: %v", err)
			} else {
				log.Printf("Webhook sent: %s on %s", event.Operation, event.Table)
			}

		case <-time.After(90 * time.Second):
			go listener.Ping()
		}
	}
}

func sendWebhook(url string, event Event) error {
	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
