package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
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
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()

	// Run migrations.
	runMigrations(db)

	// Simple HTTP endpoint that demonstrates inserting data.
	// For demo purposes, we insert a new user and a related hotel record.
	http.HandleFunc("/insert", func(w http.ResponseWriter, r *http.Request) {
		// Insert a new user.
		var userID int
		err := db.QueryRow(`
			INSERT INTO users (name, email)
			VALUES ('John Doe', 'john@example.com')
			RETURNING id;
		`).Scan(&userID)
		if err != nil {
			http.Error(w, "Error inserting user", http.StatusInternalServerError)
			return
		}

		// Insert a hotel record linked to the user.
		_, err = db.Exec("INSERT INTO hotels (user_id, data) VALUES ($1, $2);", userID, "Sample Hotel Data")
		if err != nil {
			http.Error(w, "Error inserting hotel", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("Data inserted successfully"))
	})

	log.Println("Server is running on port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
