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
	"strings"
	"time"
)

const (
	apiURL  = "http://localhost:8080/api"
	tcpAddr = "localhost:9090"
	udpAddr = "localhost:9091"
)

var (
	token    string
	username string
	userID   string
)

func main() {
	fmt.Println("╔════════════════════════════════════════════╗")
	fmt.Println("║     MangaHub CLI Test Client v1.0          ║")
	fmt.Println("╚════════════════════════════════════════════╝")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n🔹 Main Menu:")
		fmt.Println("1. Register")
		fmt.Println("2. Login")
		fmt.Println("3. Search Manga")
		fmt.Println("4. View Library")
		fmt.Println("5. Add to Library")
		fmt.Println("6. Update Progress")
		fmt.Println("7. Test TCP Sync")
		fmt.Println("8. Test UDP Notifications")
		fmt.Println("9. Exit")
		fmt.Print("\n👉 Choose option: ")

		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			register(reader)
		case "2":
			login(reader)
		case "3":
			searchManga(reader)
		case "4":
			viewLibrary()
		case "5":
			addToLibrary(reader)
		case "6":
			updateProgress(reader)
		case "7":
			testTCP()
		case "8":
			testUDP()
		case "9":
			fmt.Println("👋 Goodbye!")
			return
		default:
			fmt.Println("❌ Invalid option")
		}
	}
}

func register(reader *bufio.Reader) {
	fmt.Println("\n📝 Register New Account")
	fmt.Print("Username: ")
	user, _ := reader.ReadString('\n')
	user = strings.TrimSpace(user)

	fmt.Print("Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Print("Password: ")
	pass, _ := reader.ReadString('\n')
	pass = strings.TrimSpace(pass)

	data := map[string]string{
		"username": user,
		"email":    email,
		"password": pass,
	}

	resp, err := makeRequest("POST", "/auth/register", data, "")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	fmt.Println("✅ Registration successful!")
	printJSON(resp)
}

func login(reader *bufio.Reader) {
	fmt.Println("\n🔐 Login")
	fmt.Print("Username: ")
	user, _ := reader.ReadString('\n')
	user = strings.TrimSpace(user)

	fmt.Print("Password: ")
	pass, _ := reader.ReadString('\n')
	pass = strings.TrimSpace(pass)

	data := map[string]string{
		"username": user,
		"password": pass,
	}

	resp, err := makeRequest("POST", "/auth/login", data, "")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	// Extract token
	if respData, ok := resp["data"].(map[string]interface{}); ok {
		if t, ok := respData["token"].(string); ok {
			token = t
			username = user
			if uid, ok := respData["user_id"].(string); ok {
				userID = uid
			}
			fmt.Println("✅ Login successful!")
			fmt.Printf("🎫 Token: %s...\n", token[:20])
		}
	}
}

func searchManga(reader *bufio.Reader) {
	fmt.Println("\n🔍 Search Manga")
	fmt.Print("Search query: ")
	query, _ := reader.ReadString('\n')
	query = strings.TrimSpace(query)

	url := fmt.Sprintf("%s/manga?query=%s", apiURL, query)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	fmt.Println("✅ Search Results:")
	printJSON(result)
}

func viewLibrary() {
	if token == "" {
		fmt.Println("❌ Please login first")
		return
	}

	fmt.Println("\n📚 Your Library")
	resp, err := makeRequest("GET", "/library", nil, token)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	printJSON(resp)
}

func addToLibrary(reader *bufio.Reader) {
	if token == "" {
		fmt.Println("❌ Please login first")
		return
	}

	fmt.Println("\n➕ Add to Library")
	fmt.Print("Manga ID: ")
	mangaID, _ := reader.ReadString('\n')
	mangaID = strings.TrimSpace(mangaID)

	fmt.Print("Status (reading/completed/plan-to-read): ")
	status, _ := reader.ReadString('\n')
	status = strings.TrimSpace(status)

	data := map[string]interface{}{
		"manga_id": mangaID,
		"status":   status,
		"current_chapter": 0,
	}

	resp, err := makeRequest("POST", "/library", data, token)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	fmt.Println("✅ Added to library!")
	printJSON(resp)
}

func updateProgress(reader *bufio.Reader) {
	if token == "" {
		fmt.Println("❌ Please login first")
		return
	}

	fmt.Println("\n📖 Update Progress")
	fmt.Print("Manga ID: ")
	mangaID, _ := reader.ReadString('\n')
	mangaID = strings.TrimSpace(mangaID)

	fmt.Print("Chapter: ")
	var chapter int
	fmt.Scanf("%d\n", &chapter)

	data := map[string]interface{}{
		"manga_id": mangaID,
		"chapter":  chapter,
	}

	resp, err := makeRequest("PUT", "/progress", data, token)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	fmt.Println("✅ Progress updated!")
	printJSON(resp)
}

func testTCP() {
	if userID == "" {
		fmt.Println("❌ Please login first")
		return
	}

	fmt.Println("\n🔄 Testing TCP Sync Connection...")

	conn, err := net.Dial("tcp", tcpAddr)
	if err != nil {
		fmt.Printf("❌ Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	// Send auth message
	authMsg := map[string]string{
		"user_id": userID,
	}
	authData, _ := json.Marshal(authMsg)
	conn.Write(append(authData, '\n'))

	// Read confirmation
	reader := bufio.NewReader(conn)
	response, _ := reader.ReadBytes('\n')
	fmt.Printf("✅ TCP Connection established: %s\n", string(response))

	// Listen for updates for 30 seconds
	fmt.Println("👂 Listening for progress updates (30s)...")
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			hb := map[string]string{"type": "heartbeat"}
			hbData, _ := json.Marshal(hb)
			conn.Write(append(hbData, '\n'))
		}
	}()

	for {
		data, err := reader.ReadBytes('\n')
		if err != nil {
			break
		}
		fmt.Printf("📨 Received: %s", string(data))
	}

	fmt.Println("✅ TCP test completed")
}

func testUDP() {
	if userID == "" {
		fmt.Println("❌ Please login first")
		return
	}

	fmt.Println("\n📢 Testing UDP Notifications...")

	addr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		fmt.Printf("❌ Failed to resolve address: %v\n", err)
		return
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Printf("❌ Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	// Register
	regMsg := map[string]string{
		"type":    "register",
		"user_id": userID,
	}
	regData, _ := json.Marshal(regMsg)
	conn.Write(regData)

	// Read confirmation
	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := conn.ReadFromUDP(buffer)
	if err == nil {
		fmt.Printf("✅ Registered: %s\n", string(buffer[:n]))
	}

	// Listen for notifications
	fmt.Println("👂 Listening for notifications (30s)...")
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			break
		}
		fmt.Printf("🔔 Notification: %s\n", string(buffer[:n]))
	}

	fmt.Println("✅ UDP test completed")
}

func makeRequest(method, endpoint string, data interface{}, authToken string) (map[string]interface{}, error) {
	var body io.Reader
	if data != nil {
		jsonData, _ := json.Marshal(data)
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, apiURL+endpoint, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	return result, nil
}

func printJSON(data interface{}) {
	jsonData, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(jsonData))
}