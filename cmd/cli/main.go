package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
func cmdInit() {
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
func handleAuth() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub auth <register|login|logout|status>")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "register":
		cmdAuthRegister()
	case "login":
		cmdAuthLogin()
	case "logout":
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
			cmdMangaSearchGRPC()
		} else {
			cmdMangaSearch()
		}
	case "info":
		if useGRPC {
			cmdMangaInfoGRPC()
		} else {
			cmdMangaInfo()
		}
	case "list":
		cmdMangaList()
	}
}

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
		cmdLibraryList()
	case "add":
		cmdLibraryAdd()
	case "remove":
		cmdLibraryRemove()
	}
}

func cmdLibraryList() {
	status := getFlag("--status")
	url := "/library"
	if status != "" {
		url += "?status=" + status
	}

	fmt.Println("📚 Fetching your library via HTTP...")
	resp, err := makeRequest("GET", url, nil, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Failed: %v\n", err)
		os.Exit(1)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if library, ok := data["library"].([]interface{}); ok {
			if len(library) == 0 {
				fmt.Println("Your library is empty")
				fmt.Println("\n💡 Use 'mangahub library add --manga-id <id> --status reading' to add manga")
				return
			}

			fmt.Printf("\n✓ Your Library (%d entries)\n\n", len(library))
			for i, entry := range library {
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

func cmdLibraryAdd() {
	mangaID := getFlag("--manga-id")
	status := getFlag("--status")

	if mangaID == "" || status == "" {
		fmt.Println("Usage: mangahub library add --manga-id <id> --status <status>")
		fmt.Println("Status: reading, completed, plan-to-read, on-hold, dropped")
		os.Exit(1)
	}

	data := map[string]interface{}{
		"manga_id":        mangaID,
		"status":          status,
		"current_chapter": 0,
		"rating":          0,
	}

	fmt.Printf("📚 Adding manga to library via HTTP...\n")
	_, err := makeRequest("POST", "/library", data, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Added to library successfully!")
	fmt.Println("\n💡 Use 'mangahub progress update --manga-id <id> --chapter <n>' to track progress")
}

func cmdLibraryRemove() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: mangahub library remove <manga-id>")
		os.Exit(1)
	}

	mangaID := os.Args[3]
	
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
		fmt.Println("Usage: mangahub chat join")
		os.Exit(1)
	}

	requireAuth()

	switch os.Args[2] {
	case "join":
		cmdChatJoin()
	}
}

func cmdChatJoin() {
	wsURL := fmt.Sprintf("ws://%s:%d/ws/chat?token=%s",
		config.Server.Host, config.Server.HTTPPort, config.User.Token)

	fmt.Printf("💬 Connecting to WebSocket chat server...\n")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		fmt.Printf("✗ WebSocket connection failed: %v\n", err)
		fmt.Println("\n💡 Make sure the server is running")
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Println("✓ Connected to chat successfully!")
	fmt.Println("\n💬 Chat Room - Type your message (or /quit to exit)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Read messages
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var msg map[string]interface{}
			json.Unmarshal(message, &msg)

			if msgType, ok := msg["type"].(string); ok && msgType == "history" {
				if messages, ok := msg["messages"].([]interface{}); ok && len(messages) > 0 {
					fmt.Println("📜 Recent chat history:")
					for _, m := range messages {
						msgData := m.(map[string]interface{})
						username := msgData["username"].(string)
						text := msgData["message"].(string)
						ts := int64(msgData["timestamp"].(float64))
						timestamp := time.Unix(ts, 0).Format("15:04")
						fmt.Printf("[%s] %s: %s\n", timestamp, username, text)
					}
					fmt.Println()
				}
				continue
			}

			if username, ok := msg["username"].(string); ok {
				text, _ := msg["message"].(string)
				ts := int64(msg["timestamp"].(float64))
				timestamp := time.Unix(ts, 0).Format("15:04")
				
				if username == "System" {
					fmt.Printf("ℹ️  [%s] %s\n", timestamp, text)
				} else {
					fmt.Printf("[%s] %s: %s\n", timestamp, username, text)
				}
			}
		}
	}()

	// Send messages
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()

		if text == "/quit" {
			fmt.Println("\n✓ Left chat")
			return
		}

		if text == "" {
			continue
		}

		msg := map[string]interface{}{
			"type":    "chat",
			"message": text,
		}
		msgData, _ := json.Marshal(msg)
		conn.WriteMessage(websocket.TextMessage, msgData)
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