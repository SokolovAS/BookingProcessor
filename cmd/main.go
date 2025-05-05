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

const (
	batchSize    = 50
	batchTimeout = 200 * time.Millisecond
)

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

	pgConnection.SetMaxOpenConns(perPod)
	pgConnection.SetMaxIdleConns(perPod / 2)
	pgConnection.SetConnMaxLifetime(15 * time.Minute)

	// ── Prometheus metrics ────────────────────────────────────
	requestsCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "booking_processor_requests_total",
		Help: "Total bookings processed from queue",
	})
	prometheus.MustRegister(requestsCounter)

	// ── Application services ──────────────────────────────────
	userRepo := repository.NewUserRepository(pgConnection)
	hotelRepo := repository.NewHotelRepository(pgConnection)
	bookingRepo := repository.NewBookingRepo(pgConnection, userRepo, hotelRepo)
	bookingService := services.NewBookingService(bookingRepo)

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

	if _, err := ch.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		log.Fatalf("QueueDeclare: %v", err)
	}
	if err := ch.Qos(batchSize, 0, false); err != nil {
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
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(":6060", mux))
	}()

	// ── Graceful shutdown ──────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// ── Batched consumer loop ──────────────────────────────────
	go func() {
		batch := make([]BookingMessage, 0, batchSize)
		deliveries := make([]amqp.Delivery, 0, batchSize)
		timer := time.NewTimer(batchTimeout)
		defer timer.Stop()

		processBatch := func() {
			n := len(batch)
			if n == 0 {
				return
			}

			// Try registering each message
			for i, m := range batch {
				if err := bookingService.Register(m.UserEmail); err != nil {
					log.Printf("Register failed for index %d (email %s): %v", i, m.UserEmail, err)
					// On failure, requeue entire batch
					for _, d := range deliveries {
						d.Nack(false, true)
					}
					batch = batch[:0]
					deliveries = deliveries[:0]
					return
				}
			}

			// All succeeded: Ack all
			for _, d := range deliveries {
				d.Ack(false)
			}
			requestsCounter.Add(float64(n))
			log.Printf("Processed and ACKed batch of %d messages", n)

			batch = batch[:0]
			deliveries = deliveries[:0]
		}

		for {
			select {
			case d, ok := <-msgs:
				if !ok {
					// channel closed: flush and exit
					processBatch()
					cancel()
					return
				}
				var m BookingMessage
				if err := json.Unmarshal(d.Body, &m); err != nil {
					log.Printf("Invalid JSON, dropping: %v", err)
					d.Nack(false, false)
					continue
				}
				batch = append(batch, m)
				deliveries = append(deliveries, d)

				if len(batch) >= batchSize {
					if !timer.Stop() {
						<-timer.C
					}
					processBatch()
					timer.Reset(batchTimeout)
				}

			case <-timer.C:
				processBatch()
				timer.Reset(batchTimeout)

			case <-sigCh:
				// On SIGINT/SIGTERM flush remaining and exit
				processBatch()
				cancel()
				return

			case <-ctx.Done():
				return
			}
		}
	}()

	log.Println("BookingProcessor consumer running, awaiting messages…")
	<-ctx.Done()
	log.Println("Shutdown complete")
}
