package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"mangahub/internal/auth"
	grpcServer "mangahub/internal/grpc"
	"mangahub/internal/manga"
	"mangahub/internal/tcp"
	"mangahub/internal/udp"
	"mangahub/internal/user"
	ws "mangahub/internal/websocket"
	"mangahub/pkg/database"
	"mangahub/pkg/models"
	pb "mangahub/proto/proto"
	"google.golang.org/grpc"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	// Configuration
	dbPath := getEnv("DB_PATH", "./data/mangahub.db")
	jwtSecret := getEnv("JWT_SECRET", "your-secret-key-change-this")
	httpPort := getEnv("HTTP_PORT", ":8080")
	tcpPort := getEnv("TCP_PORT", ":9090")
	udpPort := getEnv("UDP_PORT", ":9091")
	grpcPort := getEnv("GRPC_PORT", ":9092")

	log.Println("╔════════════════════════════════════════════════════════════╗")
	log.Println("║           🚀 MangaHub Server Suite Starting...            ║")
	log.Println("╚════════════════════════════════════════════════════════════╝")

	// Initialize database
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Printf("✅ Database initialized: %s", dbPath)

	// Seed data
	if err := database.SeedData(db); err != nil {
		log.Printf("⚠️  Failed to seed data: %v", err)
	} else {
		log.Println("✅ Database seeded with initial data")
	}

	// Initialize repositories
	userRepo := user.NewRepository(db)
	mangaRepo := manga.NewRepository(db)

	// Initialize services
	userService := user.NewService(userRepo, jwtSecret)

	// Create progress broadcast channel
	progressBroadcast := make(chan models.ProgressUpdate, 100)

	// Initialize WebSocket hub
	chatHub := ws.NewHub()
	go chatHub.Run()
	log.Println("✅ WebSocket Chat Hub initialized")

	// Start TCP Server
	tcpServer := tcp.NewServer(tcpPort)
	if err := tcpServer.Start(); err != nil {
		log.Fatalf("❌ TCP server failed to start: %v", err)
	}
	log.Printf("✅ TCP Sync Server started on %s", tcpPort)

	// Connect TCP broadcast to HTTP API
	go func() {
		for update := range progressBroadcast {
			tcpServer.GetBroadcastChannel() <- update
		}
	}()

	// Start UDP Server
	udpServer := udp.NewServer(udpPort)
	if err := udpServer.Start(); err != nil {
		log.Fatalf("❌ UDP server failed to start: %v", err)
	}
	log.Printf("✅ UDP Notification Server started on %s", udpPort)

	// Initialize handlers WITH UDP server
	userHandler := user.NewHandler(userService)
	mangaHandler := manga.NewHandler(mangaRepo, progressBroadcast, udpServer)

	// Start gRPC Server
	go func() {
		lis, err := net.Listen("tcp", grpcPort)
		if err != nil {
			log.Fatalf("❌ gRPC listen failed: %v", err)
		}

		grpcSrv := grpc.NewServer()
		server := grpcServer.NewServer(mangaRepo)
		pb.RegisterMangaServiceServer(grpcSrv, server)

		log.Printf("✅ gRPC Internal Service started on %s", grpcPort)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("❌ gRPC server failed: %v", err)
		}
	}()

	// Setup HTTP API Server
	router := gin.Default()

	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
			"services": gin.H{
				"http":      "running",
				"tcp":       "running",
				"udp":       "running",
				"grpc":      "running",
				"websocket": "running",
			},
		})
	})

	// Server stats
	router.GET("/stats", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"tcp":       tcpServer.GetStats(),
			"udp":       udpServer.GetStats(),
			"websocket": chatHub.GetStats(),
		})
	})

	// Public routes
	public := router.Group("/api")
	{
		public.POST("/auth/register", userHandler.Register)
		public.POST("/auth/login", userHandler.Login)
		public.GET("/manga", mangaHandler.SearchManga)
		public.GET("/manga/:id", mangaHandler.GetManga)
	}

	// Protected routes
	protected := router.Group("/api")
	protected.Use(auth.JWTMiddleware(jwtSecret))
	{
		protected.GET("/users/profile", userHandler.GetProfile)
		protected.GET("/library", mangaHandler.GetLibrary)
		protected.POST("/library", mangaHandler.AddToLibrary)
		protected.DELETE("/library/:id", mangaHandler.RemoveFromLibrary)
		protected.PUT("/progress", mangaHandler.UpdateProgress)
	}

	// WebSocket route
	router.GET("/ws/chat", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
			return
		}

		claims, err := auth.ValidateToken(token, jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("Failed to upgrade: %v", err)
			return
		}

		ws.ServeWs(chatHub, conn, claims.UserID, claims.Username)
	})

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n🛑 Shutting down MangaHub servers gracefully...")
		
		// Shutdown TCP server
		tcpServer.Shutdown()
		
		// Close channels
		close(progressBroadcast)
		
		log.Println("✅ All servers shut down successfully")
		os.Exit(0)
	}()

	// Print server info
	log.Println("╔════════════════════════════════════════════════════════════╗")
	log.Println("║              ✨ All Services Running ✨                    ║")
	log.Println("╠════════════════════════════════════════════════════════════╣")
	log.Printf("║ 🌐 HTTP API:      http://localhost%s                     ║\n", httpPort)
	log.Printf("║ 🔄 TCP Sync:      tcp://localhost%s                      ║\n", tcpPort)
	log.Printf("║ 📢 UDP Notify:    udp://localhost%s                      ║\n", udpPort)
	log.Printf("║ ⚡ gRPC Service:  grpc://localhost%s                     ║\n", grpcPort)
	log.Printf("║ 💬 WebSocket:     ws://localhost%s/ws/chat              ║\n", httpPort)
	log.Println("╠════════════════════════════════════════════════════════════╣")
	log.Println("║ 📊 Health Check:  http://localhost:8080/health            ║")
	log.Println("║ 📈 Statistics:    http://localhost:8080/stats             ║")
	log.Println("╚════════════════════════════════════════════════════════════╝")

	// Start HTTP server
	log.Printf("✅ HTTP API Server started on %s\n", httpPort)
	if err := router.Run(httpPort); err != nil {
		log.Fatalf("❌ Failed to start HTTP server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}