package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"mangahub/internal/tcp"
)

func main() {
	port := getEnv("TCP_PORT", ":9090")

	// Create TCP server
	server := tcp.NewServer(port)

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down TCP server...")
		os.Exit(0)
	}()

	// Start server
	log.Printf("🔄 TCP Progress Sync Server starting on %s", port)
	log.Println("📡 Ready to accept client connections")
	log.Println("💾 Progress updates will be broadcasted to connected clients")
	
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}