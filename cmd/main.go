package main

import (
	"database/sql"
	"github.com/SokolovAS/bookingprocessor/internal/Handlers"
	repository "github.com/SokolovAS/bookingprocessor/internal/Repository"
	services "github.com/SokolovAS/bookingprocessor/internal/Services"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// DROP TABLE IF EXISTS hotels CASCADE;
func runMigrations(db *sql.DB) {
	// ────────────────────────────────────
	// 1) DROP old tables (dev only)
	if _, err := db.Exec(`DROP TABLE IF EXISTS hotels CASCADE;`); err != nil {
		log.Fatalf("Error dropping hotels: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS users CASCADE;`); err != nil {
		log.Fatalf("Error dropping users: %v", err)
	}

	// Begin a transaction to keep SET + DISTRIBUTE in one session
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Error starting transaction for users: %v", err)
	}
	defer tx.Rollback()

	// 1) Create the raw users table (no constraints)
	if _, err := tx.Exec(`
    CREATE TABLE users (
      id          BIGSERIAL,
      name        TEXT          NOT NULL,
      email       TEXT          NOT NULL,
      created_at  TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP
    );
  `); err != nil {
		log.Fatalf("Error creating users table: %v", err)
	}

	// 2) Tell Citus to shard with replication_factor = 1
	if _, err := tx.Exec(`SET citus.shard_replication_factor = 1;`); err != nil {
		log.Fatalf("Error setting shard_replication_factor for users: %v", err)
	}

	// 3) Distribute the users table on the id column
	if _, err := tx.Exec(
		`SELECT create_distributed_table('users', 'id');`,
	); err != nil {
		log.Fatalf("Error distributing users table: %v", err)
	}

	// 4) Add your constraints on the coordinator
	if _, err := tx.Exec(
		`ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (id);`,
	); err != nil {
		log.Fatalf("Error adding PK on users: %v", err)
	}
	if _, err := tx.Exec(
		`ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (id, email);`,
	); err != nil {
		log.Fatalf("Error adding UNIQUE on users: %v", err)
	}

	// 3a) Create the raw table (can be outside Tx if you like)
	if _, err := tx.Exec(`
        CREATE TABLE hotels (
          id          BIGSERIAL,
          user_id     BIGINT       NOT NULL,
          data        TEXT,
          created_at  TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP
        );
    `); err != nil {
		log.Fatalf("Error creating hotels table: %v", err)
	}

	// 3b) Set replication_factor = 1 in *this* session
	if _, err := tx.Exec(
		`SET citus.shard_replication_factor = 1;`,
	); err != nil {
		log.Fatalf("Error setting shard_replication_factor: %v", err)
	}

	// 3c) Distribute & co-locate (must be in same session as the SET!)
	if _, err := tx.Exec(`
        SELECT create_distributed_table(
          'hotels',
          'user_id',
          colocate_with := 'users'
        );
    `); err != nil {
		log.Fatalf("Error distributing hotels: %v", err)
	}

	// 3d) Add PK & FK on the coordinator
	if _, err := tx.Exec(
		`ALTER TABLE hotels ADD CONSTRAINT hotels_pkey PRIMARY KEY (user_id, id);`,
	); err != nil {
		log.Fatalf("Error adding PK on hotels: %v", err)
	}
	if _, err := tx.Exec(
		`ALTER TABLE hotels
           ADD CONSTRAINT hotels_user_fk FOREIGN KEY (user_id) REFERENCES users(id);`,
	); err != nil {
		log.Fatalf("Error adding FK on hotels: %v", err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("Error committing hotels migration: %v", err)
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

	userRepo := repository.NewUserRepository(db)
	hotelRepo := repository.NewHotelRepository(db)
	bookingRepo := repository.NewBookingRepo(db, userRepo, hotelRepo)
	userService := services.NewUserService(userRepo)
	BookingService := services.NewBookingService(bookingRepo)
	graphQLHandler := Handlers.NewGraphQLHandler(userService)
	bookingHandler := Handlers.NewBookingHandler(BookingService)

	http.Handle("/graphql", graphQLHandler)
	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/insert", func(w http.ResponseWriter, r *http.Request) {
		requestsCounter.Inc()
		log.Println("Received /insert request")

		bookingHandler.Inset(w, r)
		log.Println("Response sent for /insert")
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
