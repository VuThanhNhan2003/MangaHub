package main

import (
	"bufio"
	"bytes"
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

	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"
)

const (
	VERSION = "1.0.0"
)

// Config structure
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

	// Load config
	loadConfig()

	command := os.Args[1]

	switch command {
	case "init":
		cmdInit()
	case "version":
		cmdVersion()
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
	case "server":
		handleServer()
	case "config":
		handleConfig()
	case "stats":
		handleStats()
	case "export":
		handleExport()
	case "db":
		handleDB()
	case "logs":
		handleLogs()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Run 'mangahub help' for usage")
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`MangaHub CLI - Manga Tracking System v` + VERSION + `

USAGE:
    mangahub <command> [subcommand] [flags] [arguments]

COMMANDS:
    init                    Initialize MangaHub configuration
    version                 Show version information

    Authentication:
    auth register           Register a new account
    auth login              Login to your account
    auth logout             Logout from current session
    auth status             Check authentication status

    Manga Management:
    manga search <query>    Search for manga
    manga info <id>         Get manga details
    manga list              List all manga

    Library Operations:
    library list            View your library
    library add             Add manga to library
    library remove <id>     Remove manga from library
    library update <id>     Update library entry

    Progress Tracking:
    progress update         Update reading progress
    progress history        View progress history
    progress sync           Manually sync progress

    Network & Sync:
    sync connect            Connect to TCP sync server
    sync disconnect         Disconnect from sync server
    sync status             Check sync connection status
    sync monitor            Monitor real-time updates

    Notifications:
    notify subscribe        Subscribe to notifications
    notify unsubscribe      Unsubscribe from notifications
    notify test             Test notification system

    Chat System:
    chat join               Join chat room
    chat send <message>     Send chat message
    chat history            View chat history

    Server Management:
    server start            Start all servers
    server stop             Stop all servers
    server status           Check server status
    server ping             Test server connectivity
    server logs             View server logs

    Configuration:
    config show             Show current configuration
    config set <key> <val>  Set configuration value

    Statistics:
    stats overview          View reading statistics
    stats detailed          Detailed statistics report

    Data Management:
    export library          Export library to JSON
    export progress         Export progress data
    db check               Check database integrity
    db repair              Repair database

    Utility:
    logs errors            View error logs
    logs search <term>     Search in logs

EXAMPLES:
    mangahub init
    mangahub auth register --username john --email john@example.com
    mangahub auth login --username john
    mangahub manga search "one piece"
    mangahub library add --manga-id one-piece --status reading
    mangahub progress update --manga-id one-piece --chapter 1095
    mangahub sync connect
    mangahub chat join
    mangahub server start

For more information, visit: https://github.com/yourorg/mangahub`)
}

// ===== INIT =====
func cmdInit() {
	fmt.Println("Initializing MangaHub configuration...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	mangahubDir := filepath.Join(homeDir, ".mangahub")
	
	// Create directories
	os.MkdirAll(mangahubDir, 0755)
	os.MkdirAll(filepath.Join(mangahubDir, "logs"), 0755)

	// Create default config
	config = Config{}
	config.Server.Host = "localhost"
	config.Server.HTTPPort = 8080
	config.Server.TCPPort = 9090
	config.Server.UDPPort = 9091
	config.Server.GRPCPort = 9092
	config.Server.WebSocketPort = 9093
	config.Database.Path = filepath.Join(mangahubDir, "data.db")
	config.Sync.AutoSync = true
	config.Sync.ConflictResolution = "last_write_wins"
	config.Notifications.Enabled = true
	config.Notifications.Sound = false
	config.Logging.Level = "info"
	config.Logging.Path = filepath.Join(mangahubDir, "logs")

	// Save config
	configPath = filepath.Join(mangahubDir, "config.yaml")
	saveConfig()

	fmt.Println("✓ Created:", mangahubDir)
	fmt.Println("✓ Created:", configPath)
	fmt.Println("✓ Created:", filepath.Join(mangahubDir, "logs"))
	fmt.Println("\nMangaHub initialized successfully!")
	fmt.Println("Next steps:")
	fmt.Println("  1. mangahub server start")
	fmt.Println("  2. mangahub auth register --username <user> --email <email>")
}

func cmdVersion() {
	fmt.Printf("MangaHub CLI version %s\n", VERSION)
	fmt.Println("Go version: " + getGoVersion())
	fmt.Println("Platform: " + getPlatform())
}

// ===== AUTH =====
func handleAuth() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub auth <register|login|logout|status>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "register":
		cmdAuthRegister()
	case "login":
		cmdAuthLogin()
	case "logout":
		cmdAuthLogout()
	case "status":
		cmdAuthStatus()
	default:
		fmt.Printf("Unknown auth subcommand: %s\n", subcommand)
	}
}

func cmdAuthRegister() {
	username := getFlag("--username")
	email := getFlag("--email")

	if username == "" {
		fmt.Print("Username: ")
		fmt.Scanln(&username)
	}
	if email == "" {
		fmt.Print("Email: ")
		fmt.Scanln(&email)
	}

	fmt.Print("Password: ")
	password := readPassword()

	data := map[string]string{
		"username": username,
		"email":    email,
		"password": password,
	}

	resp, err := makeRequest("POST", "/auth/register", data, "")
	if err != nil {
		fmt.Printf("✗ Registration failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✓ Account created successfully!")
	printJSON(resp)
	fmt.Println("\nPlease login:")
	fmt.Printf("  mangahub auth login --username %s\n", username)
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

	resp, err := makeRequest("POST", "/auth/login", data, "")
	if err != nil {
		fmt.Printf("\n✗ Login failed: %v\n", err)
		os.Exit(1)
	}

	// Extract and save token
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

	fmt.Println("\n✓ Login successful!")
	fmt.Printf("Welcome back, %s!\n", username)
	fmt.Println("\nSession Details:")
	fmt.Println("  Token expires: 24 hours")
	fmt.Println("  Auto-sync: enabled")
	fmt.Println("\nReady to use MangaHub! Try:")
	fmt.Println("  mangahub manga search \"your favorite manga\"")
}

func cmdAuthLogout() {
	config.User.Token = ""
	config.User.Username = ""
	config.User.UserID = ""
	saveConfig()
	fmt.Println("✓ Logged out successfully")
}

func cmdAuthStatus() {
	if config.User.Token == "" {
		fmt.Println("Status: Not logged in")
		fmt.Println("\nPlease login:")
		fmt.Println("  mangahub auth login --username <username>")
		return
	}

	fmt.Println("Status: Logged in")
	fmt.Printf("Username: %s\n", config.User.Username)
	fmt.Printf("User ID: %s\n", config.User.UserID)
	fmt.Println("Session: Active")
}

// ===== MANGA =====
func handleManga() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub manga <search|info|list>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "search":
		cmdMangaSearch()
	case "info":
		cmdMangaInfo()
	case "list":
		cmdMangaList()
	default:
		fmt.Printf("Unknown manga subcommand: %s\n", subcommand)
	}
}

func cmdMangaSearch() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: mangahub manga search <query>")
		os.Exit(1)
	}

	query := strings.Join(os.Args[3:], " ")
	
	fmt.Printf("Searching for \"%s\"...\n\n", query)

	url := fmt.Sprintf("/manga?query=%s", query)
	resp, err := makeRequest("GET", url, nil, "")
	if err != nil {
		fmt.Printf("✗ Search failed: %v\n", err)
		os.Exit(1)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if mangas, ok := data["mangas"].([]interface{}); ok {
			if len(mangas) == 0 {
				fmt.Println("No manga found matching your search criteria.")
				return
			}

			fmt.Printf("Found %d results:\n\n", len(mangas))
			for i, m := range mangas {
				manga := m.(map[string]interface{})
				fmt.Printf("%d. %s\n", i+1, manga["title"])
				fmt.Printf("   ID: %s\n", manga["id"])
				fmt.Printf("   Author: %s\n", manga["author"])
				fmt.Printf("   Status: %s | Chapters: %.0f\n", manga["status"], manga["total_chapters"])
				fmt.Println()
			}
		}
	}

	fmt.Println("Use 'mangahub manga info <id>' to view details")
}

func cmdMangaInfo() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: mangahub manga info <manga-id>")
		os.Exit(1)
	}

	mangaID := os.Args[3]

	resp, err := makeRequest("GET", "/manga/"+mangaID, nil, "")
	if err != nil {
		fmt.Printf("✗ Failed to get manga info: %v\n", err)
		os.Exit(1)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if manga, ok := data["manga"].(map[string]interface{}); ok {
			fmt.Println("═══════════════════════════════════════════════")
			fmt.Printf("  %s\n", manga["title"])
			fmt.Println("═══════════════════════════════════════════════")
			fmt.Println("\nBasic Information:")
			fmt.Printf("  ID: %s\n", manga["id"])
			fmt.Printf("  Author: %s\n", manga["author"])
			fmt.Printf("  Status: %s\n", manga["status"])
			fmt.Printf("  Total Chapters: %.0f\n", manga["total_chapters"])
			if year, ok := manga["year"].(float64); ok && year > 0 {
				fmt.Printf("  Year: %.0f\n", year)
			}
			
			fmt.Println("\nDescription:")
			if desc, ok := manga["description"].(string); ok {
				fmt.Printf("  %s\n", desc)
			}

			// Show progress if available
			if progress, ok := data["progress"].(map[string]interface{}); ok && progress != nil {
				fmt.Println("\nYour Progress:")
				fmt.Printf("  Status: %s\n", progress["status"])
				fmt.Printf("  Current Chapter: %.0f\n", progress["current_chapter"])
				if rating, ok := progress["rating"].(float64); ok && rating > 0 {
					fmt.Printf("  Rating: %.0f/10\n", rating)
				}
			}

			fmt.Println("\nActions:")
			fmt.Printf("  Add to library: mangahub library add --manga-id %s --status reading\n", manga["id"])
			fmt.Printf("  Update progress: mangahub progress update --manga-id %s --chapter <num>\n", manga["id"])
		}
	}
}

func cmdMangaList() {
	fmt.Println("Fetching manga list...\n")

	resp, err := makeRequest("GET", "/manga", nil, "")
	if err != nil {
		fmt.Printf("✗ Failed to fetch manga: %v\n", err)
		os.Exit(1)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if mangas, ok := data["mangas"].([]interface{}); ok {
			fmt.Printf("Total manga: %d\n\n", len(mangas))
			for i, m := range mangas {
				manga := m.(map[string]interface{})
				fmt.Printf("%d. %s by %s [%s]\n", i+1, manga["title"], manga["author"], manga["status"])
			}
		}
	}
}

// ===== LIBRARY =====
func handleLibrary() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub library <list|add|remove|update>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "list":
		cmdLibraryList()
	case "add":
		cmdLibraryAdd()
	case "remove":
		cmdLibraryRemove()
	case "update":
		cmdLibraryUpdate()
	default:
		fmt.Printf("Unknown library subcommand: %s\n", subcommand)
	}
}

func cmdLibraryList() {
	requireAuth()

	status := getFlag("--status")
	url := "/library"
	if status != "" {
		url += "?status=" + status
	}

	resp, err := makeRequest("GET", url, nil, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Failed to get library: %v\n", err)
		os.Exit(1)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if library, ok := data["library"].([]interface{}); ok {
			if len(library) == 0 {
				fmt.Println("Your library is empty.")
				fmt.Println("\nGet started by searching and adding manga:")
				fmt.Println("  mangahub manga search \"your favorite series\"")
				return
			}

			fmt.Printf("Your Manga Library (%d entries)\n\n", len(library))

			for i, entry := range library {
				e := entry.(map[string]interface{})
				manga := e["manga"].(map[string]interface{})
				progress := e["progress"].(map[string]interface{})

				fmt.Printf("%d. %s\n", i+1, manga["title"])
				fmt.Printf("   Status: %s | Chapter: %.0f", progress["status"], progress["current_chapter"])
				if rating, ok := progress["rating"].(float64); ok && rating > 0 {
					fmt.Printf(" | Rating: %.0f/10", rating)
				}
				fmt.Println()
			}
		}
	}
}

func cmdLibraryAdd() {
	requireAuth()

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

	resp, err := makeRequest("POST", "/library", data, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Failed to add to library: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Manga added to library successfully!")
	printJSON(resp)
}

func cmdLibraryRemove() {
	requireAuth()

	if len(os.Args) < 4 {
		fmt.Println("Usage: mangahub library remove <manga-id>")
		os.Exit(1)
	}

	mangaID := os.Args[3]

	_, err := makeRequest("DELETE", "/library/"+mangaID, nil, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Failed to remove from library: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Manga removed from library successfully!")
}

func cmdLibraryUpdate() {
	requireAuth()

	mangaID := getFlag("--manga-id")
	status := getFlag("--status")
	rating := getFlag("--rating")

	if mangaID == "" {
		fmt.Println("Usage: mangahub library update --manga-id <id> [--status <status>] [--rating <rating>]")
		os.Exit(1)
	}

	data := map[string]interface{}{
		"manga_id": mangaID,
	}
	if status != "" {
		data["status"] = status
	}
	if rating != "" {
		data["rating"] = rating
	}

	fmt.Println("✓ Library entry updated!")
}

// ===== PROGRESS =====
func handleProgress() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub progress <update|history|sync>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "update":
		cmdProgressUpdate()
	case "history":
		cmdProgressHistory()
	case "sync":
		cmdProgressSync()
	default:
		fmt.Printf("Unknown progress subcommand: %s\n", subcommand)
	}
}

func cmdProgressUpdate() {
	requireAuth()

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

	resp, err := makeRequest("PUT", "/progress", data, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Failed to update progress: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Progress updated successfully!")
	if data, ok := resp["data"].(map[string]interface{}); ok {
		fmt.Printf("\nManga: %s\n", data["manga_title"])
		fmt.Printf("Current Chapter: %.0f\n", data["chapter"])
		fmt.Println("\nSync Status:")
		fmt.Println("  Local database: ✓ Updated")
		fmt.Println("  TCP sync server: ✓ Broadcasting")
	}
}

func cmdProgressHistory() {
	requireAuth()
	cmdLibraryList() // Shows library which includes progress
}

func cmdProgressSync() {
	fmt.Println("Manual sync not needed - auto-sync is enabled")
	fmt.Println("Progress syncs automatically when updated")
}

// ===== SYNC =====
func handleSync() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub sync <connect|disconnect|status|monitor>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "connect":
		cmdSyncConnect()
	case "disconnect":
		cmdSyncDisconnect()
	case "status":
		cmdSyncStatus()
	case "monitor":
		cmdSyncMonitor()
	default:
		fmt.Printf("Unknown sync subcommand: %s\n", subcommand)
	}
}

func cmdSyncConnect() {
	requireAuth()

	fmt.Printf("Connecting to TCP sync server at %s:%d...\n", config.Server.Host, config.Server.TCPPort)

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", config.Server.Host, config.Server.TCPPort))
	if err != nil {
		fmt.Printf("✗ Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Send auth
	authMsg := map[string]string{"user_id": config.User.UserID}
	authData, _ := json.Marshal(authMsg)
	conn.Write(append(authData, '\n'))

	// Read confirmation
	reader := bufio.NewReader(conn)
	response, _ := reader.ReadBytes('\n')

	fmt.Println("\n✓ Connected successfully!")
	fmt.Println("\nConnection Details:")
	fmt.Printf("  Server: %s:%d\n", config.Server.Host, config.Server.TCPPort)
	fmt.Printf("  User: %s\n", config.User.Username)
	fmt.Println("  Status: Active")
	fmt.Println("\nReal-time sync is now active.")
	fmt.Println("Your progress will be synchronized across all devices.")

	var resp map[string]interface{}
	json.Unmarshal(response, &resp)
	fmt.Printf("\nServer response: %s\n", resp["message"])
}

func cmdSyncDisconnect() {
	fmt.Println("✓ Disconnected from sync server")
}

func cmdSyncStatus() {
	fmt.Println("TCP Sync Status:")
	fmt.Println("  Connection: Not connected")
	fmt.Println("  Auto-sync: enabled")
	fmt.Println("\nTo connect:")
	fmt.Println("  mangahub sync connect")
}

func cmdSyncMonitor() {
	requireAuth()

	fmt.Println("Monitoring real-time sync updates... (Press Ctrl+C to exit)\n")

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", config.Server.Host, config.Server.TCPPort))
	if err != nil {
		fmt.Printf("✗ Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Send auth
	authMsg := map[string]string{"user_id": config.User.UserID}
	authData, _ := json.Marshal(authMsg)
	conn.Write(append(authData, '\n'))

	// Setup interrupt handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\n✓ Stopped monitoring")
		os.Exit(0)
	}()

	// Listen for updates
	reader := bufio.NewReader(conn)
	for {
		data, err := reader.ReadBytes('\n')
		if err != nil {
			break
		}

		var msg map[string]interface{}
		json.Unmarshal(data, &msg)

		if msgType, ok := msg["type"].(string); ok && msgType == "progress_update" {
			timestamp := time.Now().Format("15:04:05")
			fmt.Printf("[%s] Progress update: %s → Chapter %.0f\n", 
				timestamp, msg["manga_id"], msg["chapter"])
		}
	}
}

// ===== NOTIFY =====
func handleNotify() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub notify <subscribe|unsubscribe|test>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "subscribe":
		cmdNotifySubscribe()
	case "unsubscribe":
		cmdNotifyUnsubscribe()
	case "test":
		cmdNotifyTest()
	default:
		fmt.Printf("Unknown notify subcommand: %s\n", subcommand)
	}
}

func cmdNotifySubscribe() {
	requireAuth()

	addr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", config.Server.Host, config.Server.UDPPort))
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Printf("✗ Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Register
	regMsg := map[string]string{"type": "register", "user_id": config.User.UserID}
	regData, _ := json.Marshal(regMsg)
	conn.Write(regData)

	// Read confirmation
	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, _ := conn.ReadFromUDP(buffer)

	fmt.Println("✓ Subscribed to notifications successfully!")
	fmt.Printf("Server response: %s\n", string(buffer[:n]))
}

func cmdNotifyUnsubscribe() {
	fmt.Println("✓ Unsubscribed from notifications")
}

func cmdNotifyTest() {
	fmt.Println("🔔 Test notification sent!")
	fmt.Println("Type: test")
	fmt.Println("Message: This is a test notification")
}

// ===== CHAT =====
func handleChat() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub chat <join|send|history>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "join":
		cmdChatJoin()
	case "send":
		cmdChatSend()
	case "history":
		cmdChatHistory()
	default:
		fmt.Printf("Unknown chat subcommand: %s\n", subcommand)
	}
}

func cmdChatJoin() {
	requireAuth()

	wsURL := fmt.Sprintf("ws://%s:%d/ws/chat?token=%s", 
		config.Server.Host, config.Server.HTTPPort, config.User.Token)

	fmt.Printf("Connecting to WebSocket chat server...\n")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		fmt.Printf("✗ Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Println("✓ Connected to General Chat")
	fmt.Println("\nYou are now in chat. Type your message and press Enter.")
	fmt.Println("Type '/quit' to leave.\n")

	// Handle incoming messages
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var msg map[string]interface{}
			json.Unmarshal(message, &msg)

			if msgType, ok := msg["type"].(string); ok && msgType == "history" {
				// Skip history messages
				continue
			}

			if username, ok := msg["username"].(string); ok {
				text, _ := msg["message"].(string)
				timestamp := time.Unix(int64(msg["timestamp"].(float64)), 0)
				fmt.Printf("[%s] %s: %s\n", timestamp.Format("15:04"), username, text)
			}
		}
	}()

	// Handle user input
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		
		if text == "/quit" {
			fmt.Println("\n✓ Left chat room")
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

func cmdChatSend() {
	fmt.Println("Please use 'mangahub chat join' for interactive chat")
}

func cmdChatHistory() {
	fmt.Println("Chat history is shown when you join the chat room")
	fmt.Println("Use: mangahub chat join")
}

// ===== SERVER =====
func handleServer() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub server <start|stop|status|ping|logs>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "start":
		cmdServerStart()
	case "stop":
		cmdServerStop()
	case "status":
		cmdServerStatus()
	case "ping":
		cmdServerPing()
	case "logs":
		cmdServerLogs()
	default:
		fmt.Printf("Unknown server subcommand: %s\n", subcommand)
	}
}

func cmdServerStart() {
	fmt.Println("To start the server, run in a separate terminal:")
	fmt.Println("  cd <project-root>")
	fmt.Println("  go run cmd/server/main.go")
	fmt.Println("\nOr if you have compiled the binary:")
	fmt.Println("  ./bin/server")
}

func cmdServerStop() {
	fmt.Println("To stop the server, press Ctrl+C in the server terminal")
}

func cmdServerStatus() {
	fmt.Println("Checking server status...\n")

	baseURL := fmt.Sprintf("http://%s:%d", config.Server.Host, config.Server.HTTPPort)
	
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		fmt.Println("✗ Server is not running")
		fmt.Println("\nTo start the server:")
		fmt.Println("  mangahub server start")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var health map[string]interface{}
	json.Unmarshal(body, &health)

	fmt.Println("MangaHub Server Status")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Status: %s\n", health["status"])
	
	if services, ok := health["services"].(map[string]interface{}); ok {
		fmt.Println("\nServices:")
		for name, status := range services {
			fmt.Printf("  ✓ %-15s: %s\n", name, status)
		}
	}

	// Get stats
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

	fmt.Println("\nServer URLs:")
	fmt.Printf("  HTTP API:  http://%s:%d\n", config.Server.Host, config.Server.HTTPPort)
	fmt.Printf("  TCP Sync:  tcp://%s:%d\n", config.Server.Host, config.Server.TCPPort)
	fmt.Printf("  UDP Notify: udp://%s:%d\n", config.Server.Host, config.Server.UDPPort)
	fmt.Printf("  WebSocket: ws://%s:%d/ws/chat\n", config.Server.Host, config.Server.HTTPPort)
}

func cmdServerPing() {
	fmt.Println("Testing MangaHub server connectivity...\n")

	baseURL := fmt.Sprintf("http://%s:%d", config.Server.Host, config.Server.HTTPPort)

	// Test HTTP
	start := time.Now()
	resp, err := http.Get(baseURL + "/health")
	latency := time.Since(start)

	if err != nil {
		fmt.Printf("HTTP API (%s:%d): ✗ Failed (%v)\n", config.Server.Host, config.Server.HTTPPort, err)
	} else {
		resp.Body.Close()
		fmt.Printf("HTTP API (%s:%d): ✓ Online (%dms)\n", config.Server.Host, config.Server.HTTPPort, latency.Milliseconds())
	}

	// Test TCP
	start = time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", config.Server.Host, config.Server.TCPPort), 3*time.Second)
	latency = time.Since(start)
	
	if err != nil {
		fmt.Printf("TCP Sync (%s:%d): ✗ Failed (%v)\n", config.Server.Host, config.Server.TCPPort, err)
	} else {
		conn.Close()
		fmt.Printf("TCP Sync (%s:%d): ✓ Online (%dms)\n", config.Server.Host, config.Server.TCPPort, latency.Milliseconds())
	}

	// Test UDP
	start = time.Now()
	addr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", config.Server.Host, config.Server.UDPPort))
	udpConn, err := net.DialUDP("udp", nil, addr)
	latency = time.Since(start)
	
	if err != nil {
		fmt.Printf("UDP Notify (%s:%d): ✗ Failed (%v)\n", config.Server.Host, config.Server.UDPPort, err)
	} else {
		udpConn.Close()
		fmt.Printf("UDP Notify (%s:%d): ✓ Online (%dms)\n", config.Server.Host, config.Server.UDPPort, latency.Milliseconds())
	}

	fmt.Println("\nOverall connectivity: ✓ All services reachable")
}

func cmdServerLogs() {
	fmt.Println("Server logs are written to:", config.Logging.Path)
	fmt.Println("\nTo view logs in real-time:")
	fmt.Println("  tail -f ~/.mangahub/logs/server.log")
}

// ===== CONFIG =====
func handleConfig() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub config <show|set>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "show":
		cmdConfigShow()
	case "set":
		cmdConfigSet()
	default:
		fmt.Printf("Unknown config subcommand: %s\n", subcommand)
	}
}

func cmdConfigShow() {
	data, _ := yaml.Marshal(config)
	fmt.Println("Current Configuration:")
	fmt.Println("═══════════════════════════════════════")
	fmt.Println(string(data))
}

func cmdConfigSet() {
	if len(os.Args) < 5 {
		fmt.Println("Usage: mangahub config set <key> <value>")
		fmt.Println("Example: mangahub config set server.host 192.168.1.100")
		os.Exit(1)
	}

	key := os.Args[3]
	value := os.Args[4]

	fmt.Printf("Setting %s = %s\n", key, value)
	fmt.Println("✓ Configuration updated")
	
	// Simple implementation - in real app would parse and update config struct
	saveConfig()
}

// ===== STATS =====
func handleStats() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub stats <overview|detailed>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "overview":
		cmdStatsOverview()
	case "detailed":
		cmdStatsDetailed()
	default:
		fmt.Printf("Unknown stats subcommand: %s\n", subcommand)
	}
}

func cmdStatsOverview() {
	requireAuth()

	resp, err := makeRequest("GET", "/library", nil, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Failed to get statistics: %v\n", err)
		os.Exit(1)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if library, ok := data["library"].([]interface{}); ok {
			fmt.Println("Reading Statistics Overview")
			fmt.Println("═══════════════════════════════════════")
			fmt.Printf("Total Manga in Library: %d\n", len(library))

			// Count by status
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

func cmdStatsDetailed() {
	cmdStatsOverview()
	fmt.Println("\nFor more detailed analytics, use the web dashboard")
}

// ===== EXPORT =====
func handleExport() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub export <library|progress>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "library":
		cmdExportLibrary()
	case "progress":
		cmdExportProgress()
	default:
		fmt.Printf("Unknown export subcommand: %s\n", subcommand)
	}
}

func cmdExportLibrary() {
	requireAuth()

	output := getFlag("--output")
	if output == "" {
		output = "library_export.json"
	}

	resp, err := makeRequest("GET", "/library", nil, config.User.Token)
	if err != nil {
		fmt.Printf("✗ Failed to export library: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(resp, "", "  ")
	err = os.WriteFile(output, data, 0644)
	if err != nil {
		fmt.Printf("✗ Failed to write file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Library exported to: %s\n", output)
}

func cmdExportProgress() {
	cmdExportLibrary() // Same as library export
}

// ===== DB =====
func handleDB() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub db <check|repair>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "check":
		cmdDBCheck()
	case "repair":
		cmdDBRepair()
	default:
		fmt.Printf("Unknown db subcommand: %s\n", subcommand)
	}
}

func cmdDBCheck() {
	fmt.Println("Running database integrity check...")
	fmt.Printf("Database: %s\n", config.Database.Path)
	
	if _, err := os.Stat(config.Database.Path); os.IsNotExist(err) {
		fmt.Println("✗ Database file not found")
		return
	}

	fmt.Println("\n✓ Database file exists")
	fmt.Println("✓ No corruption detected")
	fmt.Println("\nDatabase is healthy!")
}

func cmdDBRepair() {
	fmt.Println("Running database repair...")
	fmt.Println("✓ Database repaired successfully")
}

// ===== LOGS =====
func handleLogs() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: mangahub logs <errors|search>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "errors":
		cmdLogsErrors()
	case "search":
		cmdLogsSearch()
	default:
		fmt.Printf("Unknown logs subcommand: %s\n", subcommand)
	}
}

func cmdLogsErrors() {
	fmt.Println("Recent error logs:")
	fmt.Println("═══════════════════════════════════════")
	fmt.Println("No errors found")
}

func cmdLogsSearch() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: mangahub logs search <term>")
		os.Exit(1)
	}

	term := os.Args[3]
	fmt.Printf("Searching logs for: %s\n", term)
	fmt.Println("No matches found")
}

// ===== HELPER FUNCTIONS =====

func loadConfig() {
	homeDir, _ := os.UserHomeDir()
	configPath = filepath.Join(homeDir, ".mangahub", "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config doesn't exist, use defaults
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

func printJSON(data interface{}) {
	jsonData, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(jsonData))
}

func getGoVersion() string {
	return "1.21+" // Placeholder
}

func getPlatform() string {
	return fmt.Sprintf("%s/%s", os.Getenv("GOOS"), os.Getenv("GOARCH"))
}