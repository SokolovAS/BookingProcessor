package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// runMigrations creates the necessary tables and constraints.
func runMigrations(db *sql.DB) {
	// Create users table if it doesn't exist.
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
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
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id)
		);
	`)
	if err != nil {
		log.Fatalf("Error creating hotels table: %v", err)
	}
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

	maxConn, _ := strconv.Atoi(os.Getenv("DB_MAX_CONNECTIONS"))
	maxPods, _ := strconv.Atoi(os.Getenv("MAX_PODS"))

	perPod := maxConn / maxPods
	idle := perPod / 2

	db.SetMaxOpenConns(perPod)

	db.SetMaxIdleConns(idle)

	db.SetConnMaxLifetime(15 * time.Minute)

	runMigrations(db)

	var requestsCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "booking_processor_requests_total",
		Help: "Total requests processed",
	})

	prometheus.MustRegister(requestsCounter)

	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/insert", func(w http.ResponseWriter, r *http.Request) {
		requestsCounter.Inc()

		email := fmt.Sprintf("john+%s@example.com", uuid.New().String())

		tx, err := db.Begin()
		if err != nil {
			log.Printf("failed to begin transaction: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		defer func() {
			if err != nil {
				tx.Rollback()
			}
		}()

		var userID int

		// Insert into users table and get the new user's ID.
		err = tx.QueryRow(`
			INSERT INTO users (name, email)
			VALUES ('John Doe', $1)
			RETURNING id;
		`, email).Scan(&userID)
		if err != nil {
			log.Printf("Insert into users failed: %v", err)
			http.Error(w, "Error inserting user", http.StatusInternalServerError)
			return
		}

		// Insert a hotel record linked to the user.
		_, err = tx.Exec("INSERT INTO hotels (user_id, data) VALUES ($1, $2);", userID, "Sample Hotel Data")
		if err != nil {
			log.Printf("Insert into hotels failed: %v", err)
			http.Error(w, "Error inserting hotel", http.StatusInternalServerError)
			return
		}

		// Commit the transaction.
		if err = tx.Commit(); err != nil {
			log.Printf("Transaction commit failed: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Write([]byte("Data inserted successfully"))
	})

	log.Println("Starting pprof goroutine...")
	go func() {
		log.Println("pprof server listening on :6060")
		if err := http.ListenAndServe(":6060", nil); err != nil {
			log.Fatalf("pprof server error: %v", err)
		}
	}()

	log.Println("Server is running on port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
