package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// runMigrations creates the necessary tables if they do not exist.
func runMigrations(db *sql.DB) {
	// Create users table if it doesn't exist.
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Error creating users table: %v", err)
	}

	// Create hotels table with a foreign key to users.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS hotels (
			id SERIAL PRIMARY KEY,
			user_id INT,
			data TEXT,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id)
		);
	`)
	if err != nil {
		log.Fatalf("Error creating hotels table: %v", err)
	}
}

func insertData(db *sql.DB, email string) error {

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// In case of error, rollback the transaction.
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var userID int

	err = tx.QueryRow(`
		INSERT INTO users (name, email)
		VALUES ('John Doe', $1)
		RETURNING id;
	`, email).Scan(&userID)
	if err != nil {
		return fmt.Errorf("insert user failed: %w", err)
	}

	_, err = tx.Exec("INSERT INTO hotels (user_id, data) VALUES ($1, $2);", userID, "Sample Hotel Data")
	if err != nil {
		return fmt.Errorf("insert hotel failed: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction failed: %w", err)
	}

	return nil
}

func main() {
	log.Println("Starting BookingProcessor service...")

	// Load the DATABASE_URL from the environment.
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	// Open connection to PostgreSQL.
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()

	// Configure connection pool settings.
	maxConn, _ := strconv.Atoi(os.Getenv("DB_MAX_CONNECTIONS"))
	maxPods, _ := strconv.Atoi(os.Getenv("MAX_PODS"))
	perPod := maxConn / maxPods
	idle := perPod / 2

	db.SetMaxOpenConns(perPod)
	db.SetMaxIdleConns(idle)
	db.SetConnMaxLifetime(15 * time.Minute)

	// Run migrations.
	runMigrations(db)

	// Set up Prometheus metrics.
	requestsCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "bookingProcessor_requests_total",
		Help: "Total requests processed",
	})
	prometheus.MustRegister(requestsCounter)
	http.Handle("/metrics", promhttp.Handler())

	// Create a semaphore to limit concurrent DB operations to perPod.
	sem := make(chan struct{}, perPod)

	http.HandleFunc("/insert", func(w http.ResponseWriter, r *http.Request) {
		requestsCounter.Inc()

		email := fmt.Sprintf("john+%s@example.com", uuid.New().String())

		resultCh := make(chan error, 1)

		// Launch the DB insert in a separate goroutine.
		go func() {
			// Acquire a token.
			sem <- struct{}{}
			defer func() {
				<-sem // Release the token.
			}()

			resultCh <- insertData(db, email)
		}()

		// Wait for the insert result or timeout.
		select {
		case err := <-resultCh:
			if err != nil {
				log.Printf("Transaction failed: %v", err)
				http.Error(w, "Error inserting data", http.StatusInternalServerError)
				return
			}
		case <-time.After(5 * time.Second):
			http.Error(w, "Request timed out", http.StatusGatewayTimeout)
			return
		}

		w.Write([]byte("Data inserted successfully"))
	})

	log.Println("Server is running on port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
