package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/graphql-go/graphql"
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

	var userType = graphql.NewObject(graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.Int,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"email": &graphql.Field{
				Type: graphql.String,
			},
			"created_at": &graphql.Field{
				// Return the created time as a string (formatted in RFC3339)
				Type: graphql.String,
			},
		},
	})

	var queryType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"hello": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return "world", nil
				},
			},
			"users": &graphql.Field{
				Type: graphql.NewList(userType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					// Query all users from the database.
					rows, err := db.Query("SELECT id, name, email, created_at FROM users")
					if err != nil {
						return nil, err
					}
					defer rows.Close()

					var users []map[string]interface{}
					for rows.Next() {
						var id int
						var name, email string
						var createdAt time.Time
						if err := rows.Scan(&id, &name, &email, &createdAt); err != nil {
							return nil, err
						}

						user := map[string]interface{}{
							"id":         id,
							"name":       name,
							"email":      email,
							"created_at": createdAt.Format(time.RFC3339),
						}
						users = append(users, user)
					}
					return users, nil
				},
			},
		},
	})

	// Create the schema with our query type.
	var schema, schemaErr = graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})

	if schemaErr != nil {
		log.Fatalf("failed to create new schema, error: %v", schemaErr)
	}

	http.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		var query string

		// Support both GET (query parameter) and POST (request body)
		if r.Method == "POST" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "unable to read request body", http.StatusBadRequest)
				return
			}
			// Assume the POST body is a plain GraphQL query. For a more complete implementation, you might handle JSON payloads.
			query = string(body)
		} else {
			query = r.URL.Query().Get("query")
		}

		result := graphql.Do(graphql.Params{
			Schema:        schema,
			RequestString: query,
		})

		// Log errors if any
		if len(result.Errors) > 0 {
			log.Printf("failed to execute graphql operation, errors: %+v", result.Errors)
		}

		// Return the result as a JSON response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

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
