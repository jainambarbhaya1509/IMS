package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jainambarbhaya1509/ims/api"
	"github.com/jainambarbhaya1509/ims/ingestion"
	"github.com/jainambarbhaya1509/ims/storage"
	"github.com/jainambarbhaya1509/ims/worker"
)

func main() {
	// 1. Connect to all databases
	pg := storage.NewPostgres()
	mongo := storage.NewMongo()
	redis := storage.NewRedis()

	// 2. Create the in-memory signal buffer (channel)
	// This is the core backpressure mechanism.
	// If Postgres/Mongo are slow, signals queue here instead of crashing.
	signalBuffer := make(chan ingestion.Signal, 50000)

	// 3. Start the debounce worker in the background
	// It reads from signalBuffer and decides: new incident or group with existing?
	dw := worker.NewDebounceWorker(signalBuffer, pg, mongo, redis)
	go dw.Start()

	// 4. Start throughput logger — prints signals/sec every 5s (assignment requirement)
	go ingestion.StartMetricsLogger(dw)

	// 5. Set up HTTP routes
	router := api.NewRouter(signalBuffer, pg, mongo, redis)

	// 6. Start server with graceful shutdown
	// Graceful shutdown = finish in-flight requests before quitting
	srv := &http.Server{Addr: ":8080", Handler: router}

	go func() {
		log.Println("Server running on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Wait for Ctrl+C
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}