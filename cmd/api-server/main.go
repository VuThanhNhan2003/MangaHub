package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"mangahub/internal/auth"
	"mangahub/internal/manga"
	"mangahub/internal/user"
	ws "mangahub/internal/websocket"
	"mangahub/pkg/database"
	"mangahub/pkg/models"
	"net/http"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

func main() {
	// Configuration
	dbPath := getEnv("DB_PATH", "./data/mangahub.db")
	jwtSecret := getEnv("JWT_SECRET", "your-secret-key-change-this")
	port := getEnv("PORT", ":8080")

	// Initialize database
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Seed initial data
	if err := database.SeedData(db); err != nil {
		log.Printf("Warning: Failed to seed data: %v", err)
	}

	// Initialize repositories
	userRepo := user.NewRepository(db)
	mangaRepo := manga.NewRepository(db)

	// Initialize services
	userService := user.NewService(userRepo, jwtSecret)

	// Create progress broadcast channel (for TCP server)
	progressBroadcast := make(chan models.ProgressUpdate, 100)

	// Initialize handlers (without UDP for standalone API server)
	userHandler := user.NewHandler(userService)
	mangaHandler := manga.NewHandler(mangaRepo, progressBroadcast, nil)

	// Initialize WebSocket hub
	chatHub := ws.NewHub()
	go chatHub.Run()

	// Setup Gin router
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
			"service": "MangaHub API",
		})
	})

	// Public routes
	public := router.Group("/api")
	{
		// Auth routes
		public.POST("/auth/register", userHandler.Register)
		public.POST("/auth/login", userHandler.Login)
		
		// Public manga routes
		public.GET("/manga", mangaHandler.SearchManga)
		public.GET("/manga/:id", mangaHandler.GetManga)
	}

	// Protected routes
	protected := router.Group("/api")
	protected.Use(auth.JWTMiddleware(jwtSecret))
	{
		// User routes
		protected.GET("/users/profile", userHandler.GetProfile)
		
		// Library routes
		protected.GET("/library", mangaHandler.GetLibrary)
		protected.POST("/library", mangaHandler.AddToLibrary)
		protected.DELETE("/library/:id", mangaHandler.RemoveFromLibrary)
		
		// Progress routes
		protected.PUT("/progress", mangaHandler.UpdateProgress)
	}

	// WebSocket route (with auth)
	router.GET("/ws/chat", func(c *gin.Context) {
		// Get auth token from query params for WebSocket
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "token required",
			})
			return
		}

		// Validate token
		claims, err := auth.ValidateToken(token, jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token",
			})
			return
		}

		// Upgrade connection
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("Failed to upgrade connection: %v", err)
			return
		}

		// Serve WebSocket
		ws.ServeWs(chatHub, conn, claims.UserID, claims.Username)
	})

	// Start server
	log.Printf("🚀 MangaHub API Server starting on %s", port)
	log.Printf("📚 Database: %s", dbPath)
	log.Printf("🔐 JWT Authentication enabled")
	log.Printf("💬 WebSocket Chat enabled at ws://localhost%s/ws/chat", port)
	
	if err := router.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}