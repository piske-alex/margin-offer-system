package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/piske-alex/margin-offer-system/store"
)

func main() {
	var (
		addr = flag.String("addr", ":8080", "gRPC server address")
		help = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	log.Printf("Starting Margin Offer Store Service on %s", *addr)

	// Create in-memory store
	memoryStore := store.NewMemoryStore()

	// Create gRPC server
	grpcServer := store.NewGRPCServer(memoryStore)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in goroutine
	go func() {
		if err := grpcServer.Start(*addr); err != nil {
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Store service is running. Press Ctrl+C to stop.")
	<-sigChan

	log.Println("Shutting down store service...")
	grpcServer.Stop()

	if err := memoryStore.Close(); err != nil {
		log.Printf("Error closing store: %v", err)
	}

	log.Println("Store service stopped")
}