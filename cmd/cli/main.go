package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	pb "mangahub/proto/proto"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
)

const VERSION = "1.0.0"

type Config struct {
	Server struct {
		Host          string `yaml:"host"`
		HTTPPort      int    `yaml:"http_port"`
		TCPPort       int    `yaml:"tcp_port"`
		UDPPort       int    `yaml:"udp_port"`
		GRPCPort      int    `yaml:"grpc_port"`
		WebSocketPort int    `yaml:"websocket_port"`
	} `yaml:"server"`
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
	User struct {
		Username string `yaml:"username"`
		Token    string `yaml:"token"`
		UserID   string `yaml:"user_id"`
	} `yaml:"user"`
	Sync struct {
		AutoSync           bool   `yaml:"auto_sync"`
		ConflictResolution string `yaml:"conflict_resolution"`
	} `yaml:"sync"`
	Notifications struct {
		Enabled bool `yaml:"enabled"`
		Sound   bool `yaml:"sound"`
	} `yaml:"notifications"`
	Logging struct {
		Level string `yaml:"level"`
		Path  string `yaml:"path"`
	} `yaml:"logging"`
}

var (
	config     Config
	configPath string
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	loadConfig()
	command := os.Args[1]

	switch command {
	case "init":
		cmdInit()
	case "version":
		fmt.Printf("MangaHub CLI v%s\n", VERSION)
	case "auth":
		handleAuth()
	case "manga":
		handleManga()
	case "library":
		handleLibrary()
	case "progress":
		handleProgress()
	case "sync":
		handleSync()
	case "notify":
		handleNotify()
	case "chat":
		handleChat()
	case "grpc":
		handleGRPC()
	case "server":
		handleServer()
	case "config":
		handleConfig()
	case "stats":
		handleStats()
	case "export":
		handleExport()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`MangaHub CLI v` + VERSION + `

Commands:
  init                     Initialize configuration
  version                  Show version
  auth <login|register>    Authentication (HTTP)
  manga <search|info>      Search and view manga (HTTP/gRPC)
  library <list|add>       Manage your library (HTTP)
  progress update          Update reading progress (HTTP)
  sync <connect|monitor>   TCP synchronization
  notify <subscribe|send>  UDP notifications
  chat join                WebSocket chat
  grpc <get|search>        gRPC operations
  server <status|ping>     Server management
  config show              View configuration
  stats overview           Reading statistics
  export library           Export data
  `)
}

// ===== INIT =====
func cmdInit() { // Initialize configuration
	homeDir, _ := os.UserHomeDir()
	mangahubDir := filepath.Join(homeDir, ".mangahub")

	os.MkdirAll(mangahubDir, 0755)
	os.MkdirAll(filepath.Join(mangahubDir, "logs"), 0755)

	config.Server.Host = "localhost"
	config.Server.HTTPPort = 8080
	config.Server.TCPPort = 9090
	config.Server.UDPPort = 9091
	config.Server.GRPCPort = 9092
	config.Server.WebSocketPort = 8080
	config.Database.Path = filepath.Join(mangahubDir, "data.db")
	config.Sync.AutoSync = true
	config.Sync.ConflictResolution = "last_write_wins"
	config.Notifications.Enabled = true
	config.Logging.Level = "info"
	config.Logging.Path = filepath.Join(mangahubDir, "logs")

	configPath = filepath.Join(mangahubDir, "config.yaml")
	saveConfig()

	fmt.Println("✓ MangaHub initialized")
	fmt.Printf("  Config: %s\n", configPath)
	fmt.Println("\nNext: mangahub auth register --username <user> --email <email>")
}

// ===== AUTH (UC-001, UC-002) - HTTP =====
/*
UC-001: User Registration
Primary Actor: Manga Reader
Goal: Create a new user account
Preconditions: None
Postconditions: User account is created
Main Success Scenario: 1. User provides username, email, and password
2. System validates input format and uniqueness
3. System hashes password using bcrypt
4. System creates user record in SQLite database
5. System returns success confirmation
Alternative Flows: - A1: Username already exists - System returns error message
• A2: Invalid email format - System requests valid email
• A3: Weak password - System displays password requirements
UC-002: User Authentication
Primary Actor: Manga Reader
Goal: Login to access personalized features
Preconditions: User has valid account
Postconditions: User is authenticated with JWT token
Main Success Scenario: 1. User provides username/email and password
2. System validates credentials against database
3. System generates JWT token with user information
4. System returns token for subsequent requests
5. User can access protected endpoints
Alternative Flows: - A1: Invalid credentials - System returns authentication error
• A2: Account not found - System suggests registration
UC-003: Search Manga
Primary Actor: Manga Reader
Goal: Find manga series using search criteria
Preconditions: System has manga database populated
Postconditions: Relevant manga results are displayed
Main Success Scenario: 1. User enters search query (title or author)
2. System queries SQLite database using LIKE patterns
3. System applies basic filters (genre, status) if provided
4. System returns paginated results with basic information
5. User can select manga for detailed view
Alternative Flows: - A1: No results found - System displays “no results” message
• A2: Database error - System logs error and returns generic message
UC-004: View Manga Details
Primary Actor: Manga Reader
Goal: Access detailed information about specific manga
Preconditions: Manga exists in database
Postconditions: Complete manga information is displayed
Main Success Scenario: 1. User selects manga from search results or direct URL
2. System retrieves manga details from database
3. System displays title, author, genres, description, chapter count
4. System shows user’s current progress if logged in
5. User can add manga to library or update progress
UC-005: Add Manga to Library
Primary Actor: Manga Reader
Goal: Add manga to personal reading library
Preconditions: User is authenticated, manga exists
Postconditions: Manga is added to user’s library
Main Success Scenario: 1. User clicks “Add to Library” from manga details
2. System presents status options (Reading, Completed, Plan to Read)
3. User selects initial status and current chapter
4. System creates user_progress record in database
5. System confirms addition and updates UI
Alternative Flows: - A1: Manga already in library - System offers to update status
• A2: Database error - System logs error and shows retry option
UC-006: Update Reading Progress
Primary Actor: Manga Reader
Goal: Track current reading progress
Preconditions: Manga is in user’s library
Postconditions: Progress is updated locally and broadcasted
Main Success Scenario: 1. User updates current chapter number
2. System validates chapter number against manga metadata
3. System updates user_progress record with timestamp
4. System triggers TCP broadcast to connected clients
5. System confirms update to user
Alternative Flows: - A1: Invalid chapter number - System shows validation error
• A2: TCP server unavailable - System updates locally, queues broadcast
UC-007: Connect to TCP Sync Server
Primary Actor: TCP Client
Goal: Establish connection for real-time progress updates
Preconditions: TCP server is running
Postconditions: Client is connected and registered
Main Success Scenario: 1. Client initiates TCP connection to server
2. Server accepts connection and creates goroutine handler
3. Client sends authentication message with user credentials
4. Server validates user and adds connection to active list
5. Server confirms successful registration
Alternative Flows: - A1: Authentication fails - Server closes connection
• A2: Server at capacity - Server rejects connection with error
UC-008: Broadcast Progress Update
Primary Actor: System (Automated)
Secondary Actor: TCP Client
Goal: Notify connected clients of progress changes
Preconditions: TCP server has active connections
Postconditions: All relevant clients receive update
Main Success Scenario: 1. System receives progress update from HTTP API
2. TCP server receives broadcast message via channel
3. Server identifies connections for the specific user
4. Server sends JSON progress message to connections
5. Clients receive and process update
Alternative Flows: - A1: Client connection lost - Server removes from active list
• A2: Send fails - Server logs error and continues with other clients
UC-009: Register for UDP Notifications
Primary Actor: UDP Client
Goal: Register to receive chapter release notifications
Preconditions: UDP server is running
Postconditions: Client is registered for notifications
Main Success Scenario: 1. Client sends UDP registration packet with user preferences
2. Server receives registration and extracts client address
3. Server adds client to notification list
4. Server sends confirmation packet to client
5. Client is ready to receive notifications
UC-010: Send Chapter Release Notification
Primary Actor: System Administrator
Goal: Notify users about new chapter releases
Preconditions: UDP server has registered clients
Postconditions: Notification is broadcasted to clients
Main Success Scenario: 1. Administrator triggers notification for specific manga
2. System creates notification message with manga details
3. UDP server broadcasts message to all registered clients
4. Clients receive notification and display to users
5. System logs successful broadcast
Alternative Flows: - A1: Client unreachable - Server continues with other clients
• A2: Network error - Server logs error and retries
UC-011: Join Chat
Primary Actor: Chat User
Goal: Connect to real-time chat system
Preconditions: User is authenticated, WebSocket server running
Postconditions: User is connected to chat
Main Success Scenario: 1. User’s browser initiates WebSocket connection
2. Server upgrades HTTP connection to WebSocket
3. Client sends join message with user credentials
4. Server validates user and adds to active connections
5. Server broadcasts user join notification to other users
6. User receives recent chat history
UC-012: Send Chat Message
Primary Actor: Chat User
Goal: Send message to other connected users
Preconditions: User is connected to chat
Postconditions: Message is broadcasted to all users
Main Success Scenario: 1. User types message and clicks send
2. Client sends message via WebSocket connection
3. Server receives message and validates user
4. Server broadcasts message to all connected clients
5. All users receive and display the message
Alternative Flows: - A1: Message too long - Server returns error to sender
• A2: User not authenticated - Server rejects message
UC-013: Handle User Disconnection
Primary Actor: System (Automated)
Goal: Clean up when user leaves chat
Preconditions: User was connected to chat
Postconditions: User is removed from active connections
Main Success Scenario: 1. System detects WebSocket connection closure
2. Server removes connection from active list
3. Server broadcasts user leave notification
4. Other users see updated participant list
5. Connection resources are cleaned up
UC-014: Retrieve Manga via gRPC
Primary Actor: Internal Service
Goal: Fetch manga data through gRPC interface
Preconditions: gRPC server is running
Postconditions: Manga data is returned
Main Success Scenario: 1. Client service calls GetManga gRPC method
2. gRPC server receives request with manga ID
3. Server queries database for manga information
4. Server constructs protobuf response message
5. Server returns manga data to client
UC-015: Search Manga via gRPC
Primary Actor: Internal Service
Goal: Search manga using gRPC interface
Preconditions: gRPC server is running, database populated
Postconditions: Search results are returned
Main Success Scenario: 1. Client calls SearchManga with search criteria
2. gRPC server processes search parameters
3. Server executes database query with filters
4. Server constructs response with result list
5. Server returns paginated results to client
UC-016: Update Progress via gRPC
Primary Actor: Internal Service
Goal: Update user reading progress through gRPC
Preconditions: User and manga exist
Postconditions: Progress is updated in database
Main Success Scenario: 1. Client calls UpdateProgress with user and manga data
2. gRPC server validates request parameters
3. Server updates user_progress table
4. Server triggers TCP broadcast for real-time sync
5. Server returns success confirmation
*/
// Workflow: handleAuth -> cmdAuthRegister or cmdAuthLogin -> makeRequest -> saveConfig

func handleAuth() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub auth <register|login|logout|status>")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "register":
		cmdAuthRegister() // UC-001: User Registration
	case "login":
		cmdAuthLogin() // UC-002: User Authentication
	case "logout": // Simple logout by clearing token
		config.User = struct {
			Username string `yaml:"username"`
			Token    string `yaml:"token"`
			UserID   string `yaml:"user_id"`
		}{}
		saveConfig()
		fmt.Println("✓ Logged out")
	case "status":
		if config.User.Token == "" {
			fmt.Println("Status: Not logged in")
		} else {
			fmt.Printf("Status: Logged in as %s (UserID: %s)\n", config.User.Username, config.User.UserID)
		}
	}
}

// Workflow of UC-001: cmdAuthRegister -> Input username, email, password -> Send HTTP request to /auth/register -> Handle response
// Send HTTP request to /auth/register (see internal/user/handler.go)
func cmdAuthRegister() {
	username := getFlag("--username")
	email := getFlag("--email")

	if username == "" || email == "" {
		fmt.Println("Usage: mangahub auth register --username <name> --email <email>")
		os.Exit(1)
	}

	fmt.Print("Password: ")
	password := readPassword()

	data := map[string]string{
		"username": username,
		"email":    email,
		"password": password,
	}

	fmt.Println("\n🔄 Sending registration request via HTTP...")
	resp, err := makeRequest("POST", "/auth/register", data, "")
	if err != nil {
		fmt.Printf("✗ Registration failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Account created successfully via HTTP!")
	if respData, ok := resp["data"].(map[string]interface{}); ok {
		fmt.Printf("  User ID: %s\n", respData["user_id"])
		fmt.Printf("  Username: %s\n", respData["username"])
	}
	fmt.Printf("\nNext: mangahub auth login --username %s\n", username)
}

// Workflow of UC-002: cmdAuthLogin -> Input username, password -> Send HTTP request to /auth/login -> Handle response
// Send HTTP request to /auth/login (see internal/user/handler.go)
func cmdAuthLogin() {
	username := getFlag("--username")
	if username == "" {
		fmt.Print("Username: ")
		fmt.Scanln(&username)
	}

	fmt.Print("Password: ")
	password := readPassword()

	data := map[string]string{
		"username": username,
		"password": password,
	}

	fmt.Println("\n🔄 Authenticating via HTTP...")
	resp, err := makeRequest("POST", "/auth/login", data, "")
	if err != nil {
		fmt.Printf("✗ Login failed: %v\n", err)
		os.Exit(1)
	}

	if respData, ok := resp["data"].(map[string]interface{}); ok {
		if token, ok := respData["token"].(string); ok {
			config.User.Token = token
			config.User.Username = username
			if userID, ok := respData["user_id"].(string); ok {
				config.User.UserID = userID
			}
			saveConfig()
		}
	}

	fmt.Printf("✓ Welcome back, %s! (JWT token saved)\n", username)
	fmt.Println("\n💡 Your session is now authenticated for HTTP, TCP, gRPC, and WebSocket")
}

// ===== MANGA (UC-003, UC-004) - HTTP & gRPC =====
func handleManga() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub manga <search|info|list>")
		fmt.Println("\nOptions:")
		fmt.Println("  --use-grpc    Use gRPC instead of HTTP")
		os.Exit(1)
	}

	useGRPC := hasFlag("--use-grpc")

	switch os.Args[2] {
	case "search":
		if useGRPC {
			cmdMangaSearchGRPC() // UC-015: Search Manga via gRPC
		} else {
			cmdMangaSearch() // UC-003: Search Manga via HTTP
		}
	case "info":
		if useGRPC {
			cmdMangaInfoGRPC() // UC-014: Retrieve Manga via gRPC
		} else {
			cmdMangaInfo() // UC-004: View Manga Details via HTTP
		}
	case "list":
		cmdMangaList() // List manga in library
	}
}

// Workflow of UC-003: cmdMangaSearch -> Input query -> HTTP request to /manga?query= -> Handle response
// Send HTTP request to /manga (see internal/manga/handler.go)
func cmdMangaSearch() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: mangahub manga search <query> [--use-grpc]")
		os.Exit(1)
	}

	query := strings.Join(os.Args[3:], " ")
	// Remove --use-grpc flag from query
	query = strings.ReplaceAll(query, "--use-grpc", "")
	query = strings.TrimSpace(query)

	url := fmt.Sprintf("/manga?query=%s", query)

	fmt.Printf("🔍 Searching via HTTP: %s\n", query)
	resp, err := makeRequest("GET", url, nil, "")
	if err != nil {
		fmt.Printf("✗ Search failed: %v\n", err)
		os.Exit(1)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if mangas, ok := data["mangas"].([]interface{}); ok {
			if len(mangas) == 0 {
				fmt.Println("No results found")
				return
			}

			fmt.Printf("\n✓ Found %d results via HTTP:\n\n", len(mangas))
			for i, m := range mangas {
				manga := m.(map[string]interface{})
				fmt.Printf("%d. %s\n", i+1, manga["title"])
				fmt.Printf("   ID: %s | Author: %s | Status: %s | Chapters: %.0f\n",
					manga["id"], manga["author"], manga["status"], manga["total_chapters"])
			}
			fmt.Println("\n💡 Use 'mangahub manga info <id>' for details")
			fmt.Println("💡 Add --use-grpc to search via gRPC instead")
		}
	}
}

// Workflow of UC-015: cmdMangaSearchGRPC -> Input query -> gRPC request -> Handle response
// Send gRPC request to SearchManga (see internal/grpc/server.go)
func cmdMangaSearchGRPC() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: mangahub manga search <query> --use-grpc")
		os.Exit(1)
	}

	query := strings.Join(os.Args[3:], " ")
	query = strings.ReplaceAll(query, "--use-grpc", "")
	query = strings.TrimSpace(query)

	fmt.Printf("🔍 Searching via gRPC: %s\n", query)

	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:%d", config.Server.Host, config.Server.GRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("✗ gRPC connection failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := pb.NewMangaServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.SearchManga(ctx, &pb.SearchRequest{
		Query:  query,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		fmt.Printf("✗ gRPC request failed: %v\n", err)
		os.Exit(1)
	}

	if len(resp.Mangas) == 0 {
		fmt.Println("No results found")
		return
	}

	fmt.Printf("\n✓ Found %d results via gRPC:\n\n", len(resp.Mangas))
	for i, manga := range resp.Mangas {
		fmt.Printf("%d. %s\n", i+1, manga.Title)
		fmt.Printf("   ID: %s | Author: %s | Status: %s | Chapters: %d\n",
			manga.Id, manga.Author, manga.Status, manga.TotalChapters)
	}
}

// Workflow of UC-004: cmdMangaInfo -> Input manga ID -> HTTP request to /manga/{id} -> Handle response
// Send HTTP request to /manga/{id} (see internal/manga/handler.go)
func cmdMangaInfo() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: mangahub manga info <manga-id> [--use-grpc]")
		os.Exit(1)
	}

	mangaID := os.Args[3]

	fmt.Printf("📖 Fetching manga info via HTTP: %s\n", mangaID)
	resp, err := makeRequest("GET", "/manga/"+mangaID, nil, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Failed: %v\n", err)
		os.Exit(1)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if manga, ok := data["manga"].(map[string]interface{}); ok {
			fmt.Printf("\n%s\n", manga["title"])
			fmt.Println(strings.Repeat("=", len(manga["title"].(string))))
			fmt.Printf("ID: %s\n", manga["id"])
			fmt.Printf("Author: %s\n", manga["author"])
			fmt.Printf("Status: %s\n", manga["status"])
			fmt.Printf("Chapters: %.0f\n", manga["total_chapters"])
			if year, ok := manga["year"].(float64); ok && year > 0 {
				fmt.Printf("Year: %.0f\n", year)
			}
			if desc, ok := manga["description"].(string); ok && desc != "" {
				fmt.Printf("\n%s\n", desc)
			}

			if progress, ok := data["progress"].(map[string]interface{}); ok && progress != nil {
				fmt.Println("\n📚 Your Progress:")
				fmt.Printf("  Status: %s | Chapter: %.0f",
					progress["status"], progress["current_chapter"])
				if rating, ok := progress["rating"].(float64); ok && rating > 0 {
					fmt.Printf(" | Rating: %.0f/10", rating)
				}
				fmt.Println()
			}
		}
	}
	fmt.Println("\n💡 Add --use-grpc to fetch via gRPC instead")
}

// Workflow of UC-014: cmdMangaInfoGRPC -> Input manga ID -> gRPC request -> Handle response
// Send gRPC request to GetManga (see internal/grpc/server.go)
func cmdMangaInfoGRPC() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: mangahub manga info <manga-id> --use-grpc")
		os.Exit(1)
	}

	mangaID := os.Args[3]

	fmt.Printf("📖 Fetching manga info via gRPC: %s\n", mangaID)

	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:%d", config.Server.Host, config.Server.GRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("✗ gRPC connection failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := pb.NewMangaServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetManga(ctx, &pb.GetMangaRequest{MangaId: mangaID})
	if err != nil {
		fmt.Printf("✗ gRPC request failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n%s\n", resp.Title)
	fmt.Println(strings.Repeat("=", len(resp.Title)))
	fmt.Printf("ID: %s\n", resp.Id)
	fmt.Printf("Author: %s\n", resp.Author)
	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Chapters: %d\n", resp.TotalChapters)
	if resp.Year > 0 {
		fmt.Printf("Year: %d\n", resp.Year)
	}
	if len(resp.Genres) > 0 {
		fmt.Printf("Genres: %s\n", strings.Join(resp.Genres, ", "))
	}
	if resp.Description != "" {
		fmt.Printf("\n%s\n", resp.Description)
	}
}

// Workflow: cmdMangaList -> HTTP request to /manga -> Handle response
// Send HTTP request to /manga (see internal/manga/handler.go)
func cmdMangaList() {
	fmt.Println("📚 Fetching all manga via HTTP...")
	resp, err := makeRequest("GET", "/manga", nil, "")
	if err != nil {
		fmt.Printf("✗ Failed: %v\n", err)
		os.Exit(1)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if mangas, ok := data["mangas"].([]interface{}); ok {
			fmt.Printf("\n✓ Total manga: %d\n\n", len(mangas))
			for i, m := range mangas {
				manga := m.(map[string]interface{})
				fmt.Printf("%d. %s by %s [%s]\n",
					i+1, manga["title"], manga["author"], manga["status"])
			}
		}
	}
}

// ===== LIBRARY (UC-005) - HTTP =====
func handleLibrary() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub library <list|add|remove>")
		os.Exit(1)
	}

	requireAuth()

	switch os.Args[2] {
	case "list":
		cmdLibraryList() // List library entries
	case "add":
		cmdLibraryAdd() // UC-005: Add Manga to Library
	case "remove":
		cmdLibraryRemove() // Remove manga from library
	}
}

// Workflow: cmdLibraryList -> HTTP request to /library -> Handle response
// Send HTTP request to /library (see internal/manga/handler.go)
func cmdLibraryList() {
	status := getFlag("--status")
	url := "/library"
	if status != "" {
		url += "?status=" + status
	}

	fmt.Println("📚 Fetching your library via HTTP...")
	resp, err := makeRequest("GET", url, nil, config.User.Token) // Authenticated GET request
	if err != nil {
		fmt.Printf("✗ Failed: %v\n", err)
		os.Exit(1)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if library, ok := data["library"].([]interface{}); ok { // List of library entries
			if len(library) == 0 {
				fmt.Println("Your library is empty")
				fmt.Println("\n💡 Use 'mangahub library add --manga-id <id> --status reading' to add manga")
				return
			}

			fmt.Printf("\n✓ Your Library (%d entries)\n\n", len(library))
			for i, entry := range library { // Each entry contains manga and progress
				e := entry.(map[string]interface{})
				manga := e["manga"].(map[string]interface{})
				progress := e["progress"].(map[string]interface{})

				fmt.Printf("%d. %s\n", i+1, manga["title"])
				fmt.Printf("   Status: %s | Chapter: %.0f",
					progress["status"], progress["current_chapter"])
				if rating, ok := progress["rating"].(float64); ok && rating > 0 {
					fmt.Printf(" | Rating: %.0f/10", rating)
				}
				fmt.Println()
			}
		}
	}
}

// Workflow of UC-005: cmdLibraryAdd -> Input manga ID, status -> HTTP request to /library -> Handle response
// Send HTTP request to /library (see internal/manga/handler.go)
func cmdLibraryAdd() {
	mangaID := getFlag("--manga-id")
	status := getFlag("--status")

	if mangaID == "" || status == "" {
		fmt.Println("Usage: mangahub library add --manga-id <id> --status <status>")
		fmt.Println("Status: reading, completed, plan-to-read, on-hold, dropped")
		os.Exit(1)
	}

	data := map[string]interface{}{ // Request payload
		"manga_id":        mangaID,
		"status":          status,
		"current_chapter": 0,
		"rating":          0,
	}

	fmt.Printf("📚 Adding manga to library via HTTP...\n")
	_, err := makeRequest("POST", "/library", data, config.User.Token) // Authenticated POST request
	if err != nil {
		fmt.Printf("✗ Failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Added to library successfully!")
	fmt.Println("\n💡 Use 'mangahub progress update --manga-id <id> --chapter <n>' to track progress")
}

func cmdLibraryRemove() {
	mangaID := getFlag("--manga-id")

	if mangaID == "" {
		fmt.Println("Usage: mangahub library remove --manga-id <id>")
		os.Exit(1)
	}

	fmt.Printf("📚 Removing manga from library via HTTP...\n")
	_, err := makeRequest("DELETE", "/library/"+mangaID, nil, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Removed from library")
}

// ===== PROGRESS (UC-006) - HTTP with TCP broadcast =====
func handleProgress() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub progress <update|history>")
		os.Exit(1)
	}

	requireAuth()

	switch os.Args[2] {
	case "update":
		cmdProgressUpdate()
	case "history":
		cmdLibraryList()
	}
}

func cmdProgressUpdate() {
	mangaID := getFlag("--manga-id")
	chapter := getFlag("--chapter")

	if mangaID == "" || chapter == "" {
		fmt.Println("Usage: mangahub progress update --manga-id <id> --chapter <number>")
		os.Exit(1)
	}

	var chapterNum int
	fmt.Sscanf(chapter, "%d", &chapterNum)

	data := map[string]interface{}{
		"manga_id": mangaID,
		"chapter":  chapterNum,
	}

	fmt.Printf("📖 Updating progress via HTTP...\n")
	resp, err := makeRequest("PUT", "/progress", data, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Progress updated successfully!")
	if data, ok := resp["data"].(map[string]interface{}); ok {
		fmt.Printf("  Manga: %s\n", data["manga_title"])
		fmt.Printf("  Chapter: %.0f\n", data["chapter"])
	}

	fmt.Println("\n💡 This update will be broadcasted to all your connected TCP clients")
	fmt.Println("💡 Use 'mangahub sync monitor' to see real-time updates")
}

// ===== SYNC (UC-007, UC-008) - TCP =====
func handleSync() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub sync <connect|monitor|status>")
		os.Exit(1)
	}

	requireAuth()

	switch os.Args[2] {
	case "connect":
		cmdSyncConnect()
	case "monitor":
		cmdSyncMonitor()
	case "status":
		cmdSyncStatus()
	}
}

func cmdSyncConnect() {
	fmt.Printf("🔄 Connecting to TCP sync server at %s:%d...\n", config.Server.Host, config.Server.TCPPort)

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", config.Server.Host, config.Server.TCPPort))
	if err != nil {
		fmt.Printf("✗ TCP connection failed: %v\n", err)
		fmt.Println("\n💡 Make sure the server is running: go run cmd/server/main.go")
		os.Exit(1)
	}
	defer conn.Close()

	// Send authentication
	authMsg := map[string]string{"user_id": config.User.UserID}
	authData, _ := json.Marshal(authMsg)
	conn.Write(append(authData, '\n'))

	// Read confirmation
	reader := bufio.NewReader(conn)
	response, err := reader.ReadBytes('\n')
	if err != nil {
		fmt.Printf("✗ Failed to read response: %v\n", err)
		os.Exit(1)
	}

	var resp map[string]interface{}
	json.Unmarshal(response, &resp)

	fmt.Println("✓ Connected to TCP sync server successfully!")
	fmt.Printf("  Status: %s\n", resp["status"])
	fmt.Printf("  Message: %s\n", resp["message"])
	fmt.Printf("  Client ID: %s\n", resp["client_id"])

	fmt.Println("\n💡 Connection established. You will now receive real-time progress updates")
	fmt.Println("💡 Use 'mangahub sync monitor' to keep the connection alive and monitor updates")
}

func cmdSyncMonitor() {
	fmt.Printf("🔄 Connecting to TCP sync server for monitoring...\n")

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", config.Server.Host, config.Server.TCPPort))
	if err != nil {
		fmt.Printf("✗ TCP connection failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Authenticate
	authMsg := map[string]string{"user_id": config.User.UserID}
	authData, _ := json.Marshal(authMsg)
	conn.Write(append(authData, '\n'))

	// Read confirmation
	reader := bufio.NewReader(conn)
	confirmData, _ := reader.ReadBytes('\n')
	var confirm map[string]interface{}
	json.Unmarshal(confirmData, &confirm)

	fmt.Println("✓ Connected to TCP sync server")
	fmt.Printf("  Client ID: %s\n", confirm["client_id"])
	fmt.Println("\n📡 Monitoring real-time progress updates... (Press Ctrl+C to exit)\n")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\n✓ Disconnected from TCP sync server")
		os.Exit(0)
	}()

	// Send heartbeat every 30 seconds
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			heartbeat := map[string]string{"type": "heartbeat"}
			hbData, _ := json.Marshal(heartbeat)
			conn.Write(append(hbData, '\n'))
		}
	}()

	// Listen for updates
	for {
		data, err := reader.ReadBytes('\n')
		if err != nil {
			fmt.Printf("\n✗ Connection lost: %v\n", err)
			break
		}

		var msg map[string]interface{}
		json.Unmarshal(data, &msg)

		if msgType, ok := msg["type"].(string); ok {
			if msgType == "progress_update" {
				timestamp := time.Now().Format("15:04:05")
				fmt.Printf("🔔 [%s] Progress Update\n", timestamp)
				fmt.Printf("   Manga ID: %v\n", msg["manga_id"])
				fmt.Printf("   Chapter: %.0f\n", msg["chapter"])
				fmt.Printf("   Timestamp: %v\n\n", msg["timestamp"])
			} else if msgType == "heartbeat_ack" {
				// Silent heartbeat acknowledgment
			}
		}
	}
}

func cmdSyncStatus() {
	fmt.Println("TCP Sync Status:")
	fmt.Println("================")
	fmt.Printf("Server: %s:%d\n", config.Server.Host, config.Server.TCPPort)
	fmt.Printf("User ID: %s\n", config.User.UserID)
	fmt.Printf("Auto-sync: %v\n", config.Sync.AutoSync)
	fmt.Println("\n💡 Use 'mangahub sync connect' to test connection")
	fmt.Println("💡 Use 'mangahub sync monitor' to watch real-time updates")
}

// ===== NOTIFY (UC-009, UC-010) - UDP =====
func handleNotify() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub notify <subscribe|test|send>")
		os.Exit(1)
	}

	requireAuth()

	switch os.Args[2] {
	case "subscribe":
		cmdNotifySubscribe()
	case "test":
		cmdNotifyTest()
	case "send":
		cmdNotifySend()
	}
}

func cmdNotifySubscribe() {
	fmt.Printf("📢 Connecting to UDP notification server at %s:%d...\n", config.Server.Host, config.Server.UDPPort)

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", config.Server.Host, config.Server.UDPPort))
	if err != nil {
		fmt.Printf("✗ Resolve failed: %v\n", err)
		os.Exit(1)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Printf("✗ Connection failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Register
	regMsg := map[string]interface{}{
		"type":    "register",
		"user_id": config.User.UserID,
		"preferences": map[string]bool{
			"chapter_releases": config.Notifications.Enabled,
			"system_updates":   true,
		},
	}
	data, _ := json.Marshal(regMsg)
	n, err := conn.Write(data)
	if err != nil {
		fmt.Printf("✗ Failed to register: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Sent registration (%d bytes)\n", n)

	// Wait for confirmation
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buffer := make([]byte, 2048)
	n, _, err = conn.ReadFromUDP(buffer)
	if err != nil {
		fmt.Printf("✗ No confirmation: %v\n", err)
		os.Exit(1)
	}

	var confirmMsg map[string]interface{}
	json.Unmarshal(buffer[:n], &confirmMsg)

	fmt.Println("✓ Subscribed to UDP notifications successfully!")
	if msg, ok := confirmMsg["message"].(string); ok {
		fmt.Printf("  %s\n", msg)
	}

	fmt.Println("\n🔔 Listening for notifications... (Press Ctrl+C to exit)\n")

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		unreg := map[string]interface{}{"type": "unregister", "user_id": config.User.UserID}
		b, _ := json.Marshal(unreg)
		conn.Write(b)
		time.Sleep(100 * time.Millisecond)
		fmt.Println("\n✓ Unsubscribed from notifications")
		os.Exit(0)
	}()

	// Keep-alive
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			ping := map[string]interface{}{"type": "ping"}
			pingData, _ := json.Marshal(ping)
			conn.Write(pingData)
		}
	}()

	// Listen
	conn.SetReadDeadline(time.Time{})
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Printf("\n✗ Error: %v\n", err)
			return
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(buffer[:n], &msg); err != nil {
			continue
		}

		if msgType, ok := msg["type"].(string); ok && msgType == "pong" {
			continue
		}

		if title, ok := msg["title"].(string); ok {
			timestamp := time.Now().Format("15:04:05")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Printf("🔔 [%s] %s\n", timestamp, title)
			if message, ok := msg["message"].(string); ok {
				fmt.Printf("   %s\n", message)
			}
			if mangaTitle, ok := msg["manga_title"].(string); ok {
				fmt.Printf("   📖 %s\n", mangaTitle)
			}
			if chapter, ok := msg["chapter"].(float64); ok {
				fmt.Printf("   📑 Chapter %.0f\n", chapter)
			}
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		}
	}
}

func cmdNotifyTest() {
	fmt.Printf("🧪 Testing UDP connection to %s:%d...\n", config.Server.Host, config.Server.UDPPort)

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", config.Server.Host, config.Server.UDPPort))
	if err != nil {
		fmt.Printf("✗ Resolve failed: %v\n", err)
		return
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Printf("✗ Connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	msg := map[string]interface{}{"type": "ping"}
	data, _ := json.Marshal(msg)

	start := time.Now()
	conn.Write(data)

	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		fmt.Printf("✗ No response: %v\n", err)
		return
	}

	var resp map[string]interface{}
	json.Unmarshal(buffer[:n], &resp)

	if resp["type"] == "pong" {
		fmt.Printf("✓ UDP communication successful! (%d ms)\n", time.Since(start).Milliseconds())
	}
}

func cmdNotifySend() {
	mangaID := getFlag("--manga-id")
	chapterStr := getFlag("--chapter")

	if mangaID == "" || chapterStr == "" {
		fmt.Println("Usage: mangahub notify send --manga-id <id> --chapter <number>")
		os.Exit(1)
	}

	var chapter int
	fmt.Sscanf(chapterStr, "%d", &chapter)

	data := map[string]interface{}{
		"manga_id": mangaID,
		"chapter":  chapter,
	}

	fmt.Println("📢 Sending notification via HTTP API...")
	resp, err := makeRequest("POST", "/notify/chapter", data, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Notification sent successfully!")
	if respData, ok := resp["data"].(map[string]interface{}); ok {
		fmt.Printf("  Manga: %s\n", respData["manga_title"])
		fmt.Printf("  Chapter: %.0f\n", respData["chapter"])
	}
	fmt.Println("\n💡 All subscribed UDP clients will receive this notification")
}

// ===== CHAT (UC-011, UC-012, UC-013) - WebSocket =====
func handleChat() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  mangahub chat join [room]       - Join a chat room (default: general)")
		fmt.Println("  mangahub chat rooms             - List available rooms")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "join":
		cmdChatJoin()
	case "rooms":
		cmdChatRooms()
	default:
		fmt.Println("Unknown command. Use: join, rooms")
		os.Exit(1)
	}
}

func cmdChatRooms() {
	requireAuth()

	url := fmt.Sprintf("http://%s:%d/stats", config.Server.Host, config.Server.HTTPPort)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+config.User.Token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("✗ Failed to get room list: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("✗ Server error: %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var stats map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&stats)

	wsStats := stats["websocket"].(map[string]interface{})
	rooms := wsStats["rooms"].([]interface{})

	fmt.Println("📋 Active Chat Rooms:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if len(rooms) == 0 {
		fmt.Println("  No active rooms")
	} else {
		for _, r := range rooms {
			room := r.(map[string]interface{})
			name := room["name"].(string)
			clients := int(room["clients"].(float64))
			fmt.Printf("  • %s (%d users)\n", name, clients)
		}
	}
	fmt.Println()
}

func cmdChatJoin() {
	// Get room from command line or use default
	room := "general"
	if len(os.Args) >= 4 {
		room = os.Args[3]
	}

	// Get username from config or prompt
	username := config.User.Username
	if username == "" {
		fmt.Print("Enter your username: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			username = strings.TrimSpace(scanner.Text())
		}
		if username == "" {
			fmt.Println("✗ Username is required")
			os.Exit(1)
		}
	}

	// Build WebSocket URL with query parameters
	wsURL := fmt.Sprintf("ws://%s:%d/ws?username=%s&room=%s",
		config.Server.Host, config.Server.HTTPPort, username, room) // Tạo room nếu chưa tồn tại

	// Connect to WebSocket server
	fmt.Printf("💬 Connecting to room '%s' as '%s'...\n", room, username)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		fmt.Printf("✗ WebSocket connection failed: %v\n", err)
		fmt.Println("\n💡 Make sure the server is running")
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("✓ Connected to room '%s' successfully!\n", room)
	fmt.Println("\n💬 Chat Room - Type your message and press Enter")
	fmt.Println("   Commands: /quit to exit, /help for help")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Channel for interrupt signal (Ctrl+C)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Channel to signal when reading is done
	done := make(chan struct{})

	// Goroutine to read messages from server
	go func() {
		defer close(done)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				log.Println("Connection closed:", err)
				return
			}

			// Parse JSON message
			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Printf("Failed to parse message: %v", err)
				continue
			}

			// Handle history message
			if msgType, ok := msg["type"].(string); ok && msgType == "history" {
				if messages, ok := msg["messages"].([]interface{}); ok && len(messages) > 0 {
					fmt.Println("📜 Recent chat history:")
					for _, m := range messages {
						msgData := m.(map[string]interface{})
						displayMessage(msgData)
					}
					fmt.Println()
				}
				continue
			}

			// Display regular message
			displayMessage(msg)
		}
	}()

	// Read input from user and send to server
	scanner := bufio.NewScanner(os.Stdin)
	go func() {
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text == "" {
				continue
			}

			// Handle commands
			if text == "/quit" {
				fmt.Println("\n✓ Left chat room")
				conn.WriteMessage(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				)
				os.Exit(0)
			}

			if text == "/help" {
				fmt.Println("\n📖 Available Commands:")
				fmt.Println("  /quit  - Exit the chat room")
				fmt.Println("  /help  - Show this help message")
				fmt.Println()
				continue
			}

			// Create message
			msg := map[string]interface{}{
				"text": text,
			}

			// Marshal to JSON
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("Failed to marshal message: %v", err)
				continue
			}

			// Send message to server
			err = conn.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				log.Println("Write error:", err)
				return
			}
		}
	}()

	// Wait for interrupt signal or connection close
	select {
	case <-done:
		fmt.Println("\n✓ Server closed the connection")
	case <-interrupt:
		fmt.Println("\n\n✓ Shutting down gracefully...")
		err := conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		if err != nil {
			log.Println("Write close error:", err)
		}
	}
}

func displayMessage(msg map[string]interface{}) {
	msgType, _ := msg["type"].(string)
	username, _ := msg["username"].(string)
	text, _ := msg["text"].(string)
	timeStr, _ := msg["time"].(string)

	switch msgType {
	case "chat":
		fmt.Printf("[%s] %s: %s\n", timeStr, username, text)
	case "system":
		fmt.Printf("[%s] * %s\n", timeStr, text)
	default:
		// Fallback for any message format
		fmt.Printf("[%s] %s: %s\n", timeStr, username, text)
	}
}

// ===== GRPC (UC-014, UC-015, UC-016) =====
func handleGRPC() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub grpc <get|search|update>")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "get":
		cmdGRPCGet()
	case "search":
		cmdGRPCSearch()
	case "update":
		cmdGRPCUpdate()
	}
}

func cmdGRPCGet() {
	mangaID := getFlag("--manga-id")
	if mangaID == "" {
		fmt.Println("Usage: mangahub grpc get --manga-id <id>")
		os.Exit(1)
	}

	fmt.Printf("📖 Fetching manga via gRPC: %s\n", mangaID)

	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:%d", config.Server.Host, config.Server.GRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("✗ gRPC connection failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := pb.NewMangaServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetManga(ctx, &pb.GetMangaRequest{MangaId: mangaID})
	if err != nil {
		fmt.Printf("✗ gRPC request failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ Success via gRPC!\n\n")
	fmt.Printf("%s\n", resp.Title)
	fmt.Println(strings.Repeat("=", len(resp.Title)))
	fmt.Printf("ID: %s\n", resp.Id)
	fmt.Printf("Author: %s\n", resp.Author)
	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Chapters: %d\n", resp.TotalChapters)
	if resp.Year > 0 {
		fmt.Printf("Year: %d\n", resp.Year)
	}
	if len(resp.Genres) > 0 {
		fmt.Printf("Genres: %s\n", strings.Join(resp.Genres, ", "))
	}
	if resp.Description != "" {
		fmt.Printf("\n%s\n", resp.Description)
	}
}

func cmdGRPCSearch() {
	query := getFlag("--query")
	if query == "" {
		fmt.Println("Usage: mangahub grpc search --query <text>")
		os.Exit(1)
	}

	fmt.Printf("🔍 Searching via gRPC: %s\n", query)

	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:%d", config.Server.Host, config.Server.GRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("✗ gRPC connection failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := pb.NewMangaServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.SearchManga(ctx, &pb.SearchRequest{
		Query:  query,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		fmt.Printf("✗ gRPC request failed: %v\n", err)
		os.Exit(1)
	}

	if len(resp.Mangas) == 0 {
		fmt.Println("No results found")
		return
	}

	fmt.Printf("\n✓ Found %d results via gRPC:\n\n", len(resp.Mangas))
	for i, manga := range resp.Mangas {
		fmt.Printf("%d. %s\n", i+1, manga.Title)
		fmt.Printf("   ID: %s | Author: %s | Status: %s | Chapters: %d\n",
			manga.Id, manga.Author, manga.Status, manga.TotalChapters)
	}
}

func cmdGRPCUpdate() {
	requireAuth()

	mangaID := getFlag("--manga-id")
	chapter := getFlag("--chapter")

	if mangaID == "" || chapter == "" {
		fmt.Println("Usage: mangahub grpc update --manga-id <id> --chapter <number>")
		os.Exit(1)
	}

	var chapterNum int32
	fmt.Sscanf(chapter, "%d", &chapterNum)

	fmt.Printf("📖 Updating progress via gRPC...\n")

	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:%d", config.Server.Host, config.Server.GRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("✗ gRPC connection failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := pb.NewMangaServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.UpdateProgress(ctx, &pb.UpdateProgressRequest{
		UserId:  config.User.UserID,
		MangaId: mangaID,
		Chapter: chapterNum,
	})
	if err != nil {
		fmt.Printf("✗ gRPC request failed: %v\n", err)
		os.Exit(1)
	}

	if resp.Success {
		fmt.Println("✓ Progress updated successfully via gRPC!")
		fmt.Printf("  Chapter: %d\n", resp.CurrentChapter)
		fmt.Println("\n💡 This update triggered TCP broadcast to connected clients")
	} else {
		fmt.Printf("✗ %s\n", resp.Message)
	}
}

// ===== SERVER =====
func handleServer() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub server <status|ping>")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "status":
		cmdServerStatus()
	case "ping":
		cmdServerPing()
	}
}

func cmdServerStatus() {
	baseURL := fmt.Sprintf("http://%s:%d", config.Server.Host, config.Server.HTTPPort)

	fmt.Println("🔍 Checking server status...")
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		fmt.Println("✗ Server is not running")
		fmt.Println("\n💡 Start server: go run cmd/server/main.go")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var health map[string]interface{}
	json.Unmarshal(body, &health)

	fmt.Println("\n✓ MangaHub Server Status")
	fmt.Printf("Status: %s\n", health["status"])

	if services, ok := health["services"].(map[string]interface{}); ok {
		fmt.Println("\nServices:")
		for name, status := range services {
			fmt.Printf("  ✓ %-12s: %s\n", name, status)
		}
	}

	resp2, err := http.Get(baseURL + "/stats")
	if err == nil {
		defer resp2.Body.Close()
		body2, _ := io.ReadAll(resp2.Body)
		var stats map[string]interface{}
		json.Unmarshal(body2, &stats)

		fmt.Println("\nStatistics:")
		if tcp, ok := stats["tcp"].(map[string]interface{}); ok {
			fmt.Printf("  TCP clients: %.0f\n", tcp["total_clients"])
		}
		if udp, ok := stats["udp"].(map[string]interface{}); ok {
			fmt.Printf("  UDP clients: %.0f\n", udp["total_clients"])
		}
		if ws, ok := stats["websocket"].(map[string]interface{}); ok {
			fmt.Printf("  Chat users: %.0f\n", ws["active_clients"])
		}
	}
}

func cmdServerPing() {
	fmt.Println("🏓 Pinging all server protocols...\n")

	baseURL := fmt.Sprintf("http://%s:%d", config.Server.Host, config.Server.HTTPPort)

	start := time.Now()
	resp, err := http.Get(baseURL + "/health")
	latency := time.Since(start)

	if err != nil {
		fmt.Printf("HTTP API: ✗ Offline\n")
	} else {
		resp.Body.Close()
		fmt.Printf("HTTP API: ✓ Online (%dms)\n", latency.Milliseconds())
	}

	start = time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", config.Server.Host, config.Server.TCPPort), 3*time.Second)
	latency = time.Since(start)

	if err != nil {
		fmt.Printf("TCP Sync: ✗ Offline\n")
	} else {
		conn.Close()
		fmt.Printf("TCP Sync: ✓ Online (%dms)\n", latency.Milliseconds())
	}

	start = time.Now()
	addr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", config.Server.Host, config.Server.UDPPort))
	udpConn, err := net.DialUDP("udp", nil, addr)
	latency = time.Since(start)

	if err != nil {
		fmt.Printf("UDP Notify: ✗ Offline\n")
	} else {
		udpConn.Close()
		fmt.Printf("UDP Notify: ✓ Online (%dms)\n", latency.Milliseconds())
	}

	start = time.Now()
	grpcConn, err := grpc.NewClient(
		fmt.Sprintf("%s:%d", config.Server.Host, config.Server.GRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	latency = time.Since(start)

	if err != nil {
		fmt.Printf("gRPC Service: ✗ Offline\n")
	} else {
		grpcConn.Close()
		fmt.Printf("gRPC Service: ✓ Online (%dms)\n", latency.Milliseconds())
	}
}

// ===== CONFIG =====
func handleConfig() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub config show")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "show":
		data, _ := yaml.Marshal(config)
		fmt.Println("Current Configuration:")
		fmt.Println(string(data))
	}
}

// ===== STATS =====
func handleStats() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub stats overview")
		os.Exit(1)
	}

	requireAuth()

	switch os.Args[2] {
	case "overview":
		cmdStatsOverview()
	}
}

func cmdStatsOverview() {
	fmt.Println("📊 Fetching statistics via HTTP...")
	resp, err := makeRequest("GET", "/library", nil, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Failed: %v\n", err)
		os.Exit(1)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if library, ok := data["library"].([]interface{}); ok {
			fmt.Println("\n✓ Reading Statistics")
			fmt.Println("====================")
			fmt.Printf("Total Manga: %d\n", len(library))

			statusCount := make(map[string]int)
			totalChapters := 0

			for _, entry := range library {
				e := entry.(map[string]interface{})
				progress := e["progress"].(map[string]interface{})
				status := progress["status"].(string)
				statusCount[status]++

				if ch, ok := progress["current_chapter"].(float64); ok {
					totalChapters += int(ch)
				}
			}

			fmt.Println("\nBy Status:")
			for status, count := range statusCount {
				fmt.Printf("  %-15s: %d\n", status, count)
			}

			fmt.Printf("\nTotal Chapters Read: %d\n", totalChapters)
		}
	}
}

// ===== EXPORT =====
func handleExport() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub export library")
		os.Exit(1)
	}

	requireAuth()

	switch os.Args[2] {
	case "library":
		cmdExportLibrary()
	}
}

func cmdExportLibrary() {
	output := getFlag("--output")
	if output == "" {
		output = "library_export.json"
	}

	fmt.Println("📤 Exporting library via HTTP...")
	resp, err := makeRequest("GET", "/library", nil, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Export failed: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(resp, "", "  ")
	err = os.WriteFile(output, data, 0644)
	if err != nil {
		fmt.Printf("✗ Failed to write file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Exported to: %s\n", output)
}

// ===== HELPER FUNCTIONS =====

func loadConfig() {
	homeDir, _ := os.UserHomeDir()
	configPath = filepath.Join(homeDir, ".mangahub", "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}

	yaml.Unmarshal(data, &config)
}

func saveConfig() {
	data, _ := yaml.Marshal(config)
	os.WriteFile(configPath, data, 0644)
}

func makeRequest(method, endpoint string, body interface{}, token string) (map[string]interface{}, error) {
	baseURL := fmt.Sprintf("http://%s:%d/api", config.Server.Host, config.Server.HTTPPort)
	url := baseURL + endpoint

	var reqBody io.Reader
	if body != nil {
		jsonData, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if resp.StatusCode >= 400 {
		if errMsg, ok := result["error"].(string); ok {
			return nil, fmt.Errorf("%s", errMsg)
		}
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return result, nil
}

func getFlag(flag string) string {
	for i, arg := range os.Args {
		if arg == flag && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

func hasFlag(flag string) bool {
	for _, arg := range os.Args {
		if arg == flag {
			return true
		}
	}
	return false
}

func requireAuth() {
	if config.User.Token == "" {
		fmt.Println("✗ Please login first")
		fmt.Println("  mangahub auth login --username <username>")
		os.Exit(1)
	}
}

func readPassword() string {
	var password string
	fmt.Scanln(&password)
	return password
}
