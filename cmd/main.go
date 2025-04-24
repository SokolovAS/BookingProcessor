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

func main() {
	log.Println("Starting BookingProcessor service...")

	// Load the DATABASE_URL from the environment.
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	// Open connection to PostgreSQL.
	pgConnection, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer pgConnection.Close()

	maxConn, _ := strconv.Atoi(os.Getenv("DB_MAX_CONNECTIONS"))
	maxPods, _ := strconv.Atoi(os.Getenv("MAX_PODS"))

	perPod := maxConn / maxPods
	idle := perPod / 2

	pgConnection.SetMaxOpenConns(perPod)
	pgConnection.SetMaxIdleConns(idle)
	pgConnection.SetConnMaxLifetime(15 * time.Minute)

	var requestsCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "booking_processor_requests_total",
		Help: "Total requests processed",
	})

	prometheus.MustRegister(requestsCounter)

	userRepo := repository.NewUserRepository(pgConnection)
	hotelRepo := repository.NewHotelRepository(pgConnection)
	bookingRepo := repository.NewBookingRepo(pgConnection, userRepo, hotelRepo)
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
