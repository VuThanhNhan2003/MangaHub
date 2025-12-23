package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"mangahub/internal/udp"
)

func main() {
	port := getEnv("UDP_PORT", ":9091")

	// Create UDP server
	server := udp.NewServer(port)

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down UDP server...")
		os.Exit(0)
	}()

	// Start server
	log.Printf("📢 UDP Notification Server starting on %s", port)
	log.Println("🔔 Ready to broadcast notifications")
	log.Println("📨 Clients can register to receive chapter updates")
	log.Println("💡 Use CLI or API to trigger notifications: mangahub notify send")

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start UDP server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
