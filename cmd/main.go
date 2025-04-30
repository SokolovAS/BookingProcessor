package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	repository "github.com/SokolovAS/bookingprocessor/internal/Repository"
	services "github.com/SokolovAS/bookingprocessor/internal/Services"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
)

type BookingMessage struct {
	UserID    int    `json:"user_id"`
	UserEmail string `json:"user_email"`
	HotelData string `json:"hotel_data"`
}

func main() {
	log.Println("Starting BookingProcessor (consumer) service...")

	// ── Postgres setup ─────────────────────────────────────────
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
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

	// ── Prometheus metrics ────────────────────────────────────
	requestsCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "booking_processor_requests_total",
		Help: "Total bookings processed from queue",
	})
	prometheus.MustRegister(requestsCounter)

	userRepo := repository.NewUserRepository(pgConnection)
	hotelRepo := repository.NewHotelRepository(pgConnection)
	bookingRepo := repository.NewBookingRepo(pgConnection, userRepo, hotelRepo)
	BookingService := services.NewBookingService(bookingRepo)
	
	// todo Need to move this somewhere to dataAPI service (witch I wont do) http.Handle("/graphql", graphQLHandler)

	// ── RabbitMQ setup ────────────────────────────────────────
	amqpURL := os.Getenv("AMQP_URL")
	if amqpURL == "" {
		log.Fatal("AMQP_URL is not set")
	}
	queueName := os.Getenv("QUEUE_NAME")
	if queueName == "" {
		queueName = "booking_inserts"
	}

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		log.Fatalf("Failed to dial RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// ensure queue exists
	_, err = ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		log.Fatalf("QueueDeclare: %v", err)
	}
	// limit unack'd msgs to a small batch
	if err := ch.Qos(50, 0, false); err != nil {
		log.Fatalf("Qos: %v", err)
	}

	msgs, err := ch.Consume(
		queueName,
		"",    // consumer tag
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		log.Fatalf("Consume: %v", err)
	}

	// ── HTTP & pprof ───────────────────────────────────────────
	go func() {
		log.Println("pprof and metrics listening on :6060")
		httpMux := http.NewServeMux()
		httpMux.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(":6060", httpMux))
	}()

	// ── Graceful shutdown ──────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// ── Consumer loop ──────────────────────────────────────────
	go func() {
		for d := range msgs {
			var m BookingMessage
			if err := json.Unmarshal(d.Body, &m); err != nil {
				log.Printf("Invalid JSON, Nack(false,false): %v", err)
				d.Nack(false, false) // drop
				continue
			}

			err := BookingService.Register(m.UserEmail)
			if err != nil {
				log.Printf("DB insert failed, Nack(false,true): %v", err)
				d.Nack(false, true) // requeue
				continue
			}

			d.Ack(false)
			requestsCounter.Inc()
			log.Printf("ACKed booking for user/hotel=%d", m.UserID)
		}
		// msgs channel closed
		cancel()
	}()

	log.Println("BookingProcessor consumer running, awaiting messages…")
	<-ctx.Done()
	log.Println("Shutdown complete")
}
