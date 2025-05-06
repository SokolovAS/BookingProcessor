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
	"sync"
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

func runWorker(
	ctx context.Context,
	id int,
	conn *amqp.Connection,
	queueName string,
	bookingService *services.BookingService,
	requestsCounter prometheus.Counter,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	ch, err := conn.Channel()
	if err != nil {
		log.Printf("[worker %d] channel open: %v", id, err)
		return
	}
	defer ch.Close()

	if err := ch.Qos(batchSize, 0, false); err != nil {
		log.Printf("[worker %d] Qos: %v", id, err)
		return
	}

	msgs, err := ch.Consume(
		queueName,
		"",    // consumerTag
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		log.Printf("[worker %d] Consume: %v", id, err)
		return
	}

	log.Printf("[worker %d] started", id)

	batch := make([]BookingMessage, 0, batchSize)
	deliveries := make([]amqp.Delivery, 0, batchSize)
	timer := time.NewTimer(batchTimeout)
	defer timer.Stop()

	processBatch := func() {
		n := len(batch)
		if n == 0 {
			return
		}
		// Register each message
		for i, m := range batch {
			if err := bookingService.Register(m.UserEmail); err != nil {
				log.Printf("[worker %d] Register failed index=%d email=%s: %v", id, i, m.UserEmail, err)
				for _, d := range deliveries {
					d.Nack(false, true)
				}
				batch = batch[:0]
				deliveries = deliveries[:0]
				return
			}
		}
		// Ack all on success
		for _, d := range deliveries {
			d.Ack(false)
		}
		requestsCounter.Add(float64(n))
		log.Printf("[worker %d] processed batch of %d", id, n)
		batch = batch[:0]
		deliveries = deliveries[:0]
	}

	for {
		select {
		case <-ctx.Done():
			processBatch()
			log.Printf("[worker %d] shutting down", id)
			return

		case d, ok := <-msgs:
			if !ok {
				processBatch()
				log.Printf("[worker %d] msgs channel closed", id)
				return
			}
			var m BookingMessage
			if err := json.Unmarshal(d.Body, &m); err != nil {
				log.Printf("[worker %d] invalid JSON, dropping: %v", id, err)
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
		}
	}
}

func main() {
	log.Println("Starting BookingProcessor (consumer) service...")

	// ── Postgres setup ─────────────────────────────────────────
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()

	maxConn, _ := strconv.Atoi(os.Getenv("DB_MAX_CONNECTIONS"))
	maxPods, _ := strconv.Atoi(os.Getenv("MAX_PODS"))
	perPod := maxConn / maxPods
	if perPod < 1 {
		perPod = 1
	}

	db.SetMaxOpenConns(perPod)
	db.SetMaxIdleConns(perPod / 2)
	db.SetConnMaxLifetime(15 * time.Minute)

	// ── Prometheus metrics ────────────────────────────────────
	requestsCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "booking_processor_requests_total",
		Help: "Total bookings processed from queue",
	})
	prometheus.MustRegister(requestsCounter)

	// ── Application services ──────────────────────────────────
	userRepo := repository.NewUserRepository(db)
	hotelRepo := repository.NewHotelRepository(db)
	bookingRepo := repository.NewBookingRepo(db, userRepo, hotelRepo)
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

	// ── HTTP & pprof ───────────────────────────────────────────
	go func() {
		log.Println("pprof and metrics listening on :6060")
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(":6060", mux))
	}()

	// ── Graceful shutdown ──────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// ── Worker pool based on DB connections per pod ────────────
	workerCount := perPod
	log.Printf("Starting %d workers (perPod DB connections)", workerCount)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go runWorker(ctx, i, conn, queueName, bookingService, requestsCounter, &wg)
	}

	// Wait for shutdown signal
	<-sigCh
	log.Println("Shutdown signal received; waiting for workers to finish…")
	cancel()
	wg.Wait()
	log.Println("BookingProcessor shutdown complete")
}
