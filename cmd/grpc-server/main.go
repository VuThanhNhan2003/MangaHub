package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	grpcServer "mangahub/internal/grpc"
	"mangahub/internal/manga"
	"mangahub/pkg/database"
	pb "mangahub/proto/proto"
	"google.golang.org/grpc"
)

func main() {
	port := getEnv("GRPC_PORT", ":9092")
	dbPath := getEnv("DB_PATH", "./data/mangahub.db")

	// Initialize database
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	mangaRepo := manga.NewRepository(db)

	// Create gRPC server
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcSrv := grpc.NewServer()
	server := grpcServer.NewServer(mangaRepo)
	pb.RegisterMangaServiceServer(grpcSrv, server)

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down gRPC server...")
		grpcSrv.GracefulStop()
		os.Exit(0)
	}()

	// Start server
	log.Printf("⚡ gRPC Internal Service starting on %s", port)
	log.Println("🔧 Services registered:")
	log.Println("   - GetManga")
	log.Println("   - SearchManga")
	log.Println("   - UpdateProgress")
	log.Printf("📚 Database: %s", dbPath)
	
	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}