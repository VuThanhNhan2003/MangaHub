package udp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"mangahub/pkg/models"
)

type UDPClient struct {
	Addr        *net.UDPAddr
	UserID      string
	LastSeen    time.Time
	Preferences map[string]bool // notification preferences
}

type Server struct {
	port    string
	conn    *net.UDPConn
	clients map[string]*UDPClient
	mutex   sync.RWMutex
}

func NewServer(port string) *Server {
	return &Server{
		port:    port,
		clients: make(map[string]*UDPClient),
	}
}

// Start starts the UDP notification server
func (s *Server) Start() error {
	addr, err := net.ResolveUDPAddr("udp", s.port)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to start UDP server: %w", err)
	}

	s.conn = conn
	log.Printf("UDP Notification Server listening on %s", s.port)

	// Start cleanup routine for inactive clients
	go s.cleanupInactiveClients()

	// Handle incoming messages
	buffer := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading UDP: %v", err)
			continue
		}

		go s.handleMessage(buffer[:n], clientAddr)
	}
}

// handleMessage processes incoming UDP messages
func (s *Server) handleMessage(data []byte, addr *net.UDPAddr) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Error parsing UDP message: %v", err)
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "register":
		s.handleRegister(msg, addr)
	case "unregister":
		s.handleUnregister(msg, addr)
	case "ping":
		s.handlePing(addr)
	}
}

// handleRegister registers a client for notifications
func (s *Server) handleRegister(msg map[string]interface{}, addr *net.UDPAddr) {
	userID, ok := msg["user_id"].(string)
	if !ok {
		log.Printf("Registration failed: missing user_id")
		return
	}

	clientKey := addr.String()

	// Extract preferences
	preferences := make(map[string]bool)
	if prefs, ok := msg["preferences"].(map[string]interface{}); ok {
		for k, v := range prefs {
			if boolVal, ok := v.(bool); ok {
				preferences[k] = boolVal
			}
		}
	} else {
		// Default preferences
		preferences["chapter_releases"] = true
		preferences["system_updates"] = true
	}

	s.mutex.Lock()
	s.clients[clientKey] = &UDPClient{
		Addr:        addr,
		UserID:      userID,
		LastSeen:    time.Now(),
		Preferences: preferences,
	}
	s.mutex.Unlock()

	log.Printf("UDP client registered: %s (UserID: %s) with preferences: %v", clientKey, userID, preferences)

	// Send confirmation
	response := map[string]interface{}{
		"status":      "registered",
		"message":     "Successfully registered for notifications",
		"preferences": preferences,
		"timestamp":   time.Now().Unix(),
	}
	s.sendToClient(addr, response)
}

// handleUnregister removes a client from notifications
func (s *Server) handleUnregister(msg map[string]interface{}, addr *net.UDPAddr) {
	clientKey := addr.String()

	s.mutex.Lock()
	delete(s.clients, clientKey)
	s.mutex.Unlock()

	log.Printf("UDP client unregistered: %s", clientKey)

	// Send confirmation
	response := map[string]interface{}{
		"status":  "unregistered",
		"message": "Successfully unregistered from notifications",
	}
	s.sendToClient(addr, response)
}

// handlePing responds to ping messages
func (s *Server) handlePing(addr *net.UDPAddr) {
	clientKey := addr.String()

	s.mutex.Lock()
	if client, exists := s.clients[clientKey]; exists {
		client.LastSeen = time.Now()
	}
	s.mutex.Unlock()

	response := map[string]interface{}{
		"type":      "pong",
		"timestamp": time.Now().Unix(),
	}
	s.sendToClient(addr, response)
}

// SendNotificationToUser sends notification to specific user's clients
func (s *Server) SendNotificationToUser(userID string, notification models.Notification) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	data, err := json.Marshal(notification)
	if err != nil {
		log.Printf("Error marshaling notification: %v", err)
		return
	}

	sentCount := 0
	for _, client := range s.clients {
		if client.UserID == userID {
			_, err := s.conn.WriteToUDP(data, client.Addr)
			if err != nil {
				log.Printf("Error sending notification to %s: %v", client.Addr, err)
			} else {
				sentCount++
			}
		}
	}

	log.Printf("Sent notification to %d clients for user %s", sentCount, userID)
}

// SendChapterNotification broadcasts a chapter release notification
func (s *Server) SendChapterNotification(mangaTitle string, chapter int, mangaID string) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	notification := map[string]interface{}{
		"type":        "chapter_release",
		"title":       "New Chapter Released! 🔔",
		"message":     fmt.Sprintf("%s - Chapter %d is now available", mangaTitle, chapter),
		"manga_id":    mangaID,
		"manga_title": mangaTitle,
		"chapter":     chapter,
		"timestamp":   time.Now().Unix(),
	}

	data, err := json.Marshal(notification)
	if err != nil {
		log.Printf("Error marshaling chapter notification: %v", err)
		return
	}

	sentCount := 0
	failedCount := 0
	for _, client := range s.clients {
		// Check if client wants chapter release notifications
		if enabled, exists := client.Preferences["chapter_releases"]; exists && !enabled {
			continue
		}

		_, err := s.conn.WriteToUDP(data, client.Addr)
		if err != nil {
			log.Printf("Error sending notification to %s: %v", client.Addr, err)
			failedCount++
		} else {
			sentCount++
		}
	}

	log.Printf("Chapter notification sent: %s Ch.%d - Success: %d, Failed: %d", mangaTitle, chapter, sentCount, failedCount)
}

// sendToClient sends data to a specific client
func (s *Server) sendToClient(addr *net.UDPAddr, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		return
	}

	_, err = s.conn.WriteToUDP(jsonData, addr)
	if err != nil {
		log.Printf("Error sending to client %s: %v", addr, err)
	}
}

// cleanupInactiveClients removes clients that haven't been seen recently
func (s *Server) cleanupInactiveClients() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mutex.Lock()
		now := time.Now()
		for key, client := range s.clients {
			if now.Sub(client.LastSeen) > 5*time.Minute {
				delete(s.clients, key)
				log.Printf("Removed inactive UDP client: %s", key)
			}
		}
		s.mutex.Unlock()
	}
}

// GetStats returns server statistics
func (s *Server) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	uniqueUsers := make(map[string]bool)
	for _, client := range s.clients {
		uniqueUsers[client.UserID] = true
	}

	return map[string]interface{}{
		"total_clients": len(s.clients),
		"unique_users":  len(uniqueUsers),
	}
}
