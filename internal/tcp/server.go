package tcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"mangahub/pkg/models"
)

type Client struct {
	Conn   net.Conn
	UserID string
}

type Server struct {
	port      string
	clients   map[string]*Client
	mutex     sync.RWMutex
	broadcast chan models.ProgressUpdate
}

func NewServer(port string) *Server {
	return &Server{
		port:      port,
		clients:   make(map[string]*Client),
		broadcast: make(chan models.ProgressUpdate, 100),
	}
}

// Start starts the TCP server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.port)
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %w", err)
	}

	log.Printf("TCP Sync Server listening on %s", s.port)

	// Start broadcast goroutine
	go s.handleBroadcasts()

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// GetBroadcastChannel returns the broadcast channel
func (s *Server) GetBroadcastChannel() chan models.ProgressUpdate {
	return s.broadcast
}

// handleConnection handles individual client connections
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	
	// Read authentication message
	authData, err := reader.ReadBytes('\n')
	if err != nil {
		log.Printf("Error reading auth: %v", err)
		return
	}

	var authMsg struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(authData, &authMsg); err != nil {
		log.Printf("Error parsing auth: %v", err)
		return
	}

	// Register client
	clientID := fmt.Sprintf("%s_%d", authMsg.UserID, time.Now().UnixNano())
	client := &Client{
		Conn:   conn,
		UserID: authMsg.UserID,
	}

	s.mutex.Lock()
	s.clients[clientID] = client
	s.mutex.Unlock()

	log.Printf("Client connected: %s (UserID: %s)", clientID, authMsg.UserID)

	// Send confirmation
	response := map[string]interface{}{
		"status":  "connected",
		"message": "Successfully connected to TCP sync server",
	}
	respData, _ := json.Marshal(response)
	conn.Write(append(respData, '\n'))

	// Keep connection alive and handle heartbeats
	for {
		// Set read deadline
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		
		data, err := reader.ReadBytes('\n')
		if err != nil {
			break
		}

		// Handle heartbeat or other messages
		var msg map[string]interface{}
		if err := json.Unmarshal(data, &msg); err == nil {
			if msgType, ok := msg["type"].(string); ok && msgType == "heartbeat" {
				// Send heartbeat response
				hbResp := map[string]string{"type": "heartbeat_ack"}
				hbData, _ := json.Marshal(hbResp)
				conn.Write(append(hbData, '\n'))
			}
		}
	}

	// Remove client on disconnect
	s.mutex.Lock()
	delete(s.clients, clientID)
	s.mutex.Unlock()

	log.Printf("Client disconnected: %s", clientID)
}

// handleBroadcasts listens for progress updates and broadcasts them
func (s *Server) handleBroadcasts() {
	for update := range s.broadcast {
		s.broadcastUpdate(update)
	}
}

// broadcastUpdate sends progress update to all clients of the user
func (s *Server) broadcastUpdate(update models.ProgressUpdate) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	data, err := json.Marshal(map[string]interface{}{
		"type":      "progress_update",
		"user_id":   update.UserID,
		"manga_id":  update.MangaID,
		"chapter":   update.Chapter,
		"timestamp": update.Timestamp,
	})
	if err != nil {
		log.Printf("Error marshaling update: %v", err)
		return
	}

	data = append(data, '\n')

	// Send to all clients of this user
	sentCount := 0
	for _, client := range s.clients {
		if client.UserID == update.UserID {
			client.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := client.Conn.Write(data)
			if err != nil {
				log.Printf("Error sending to client: %v", err)
			} else {
				sentCount++
			}
		}
	}

	log.Printf("Broadcasted progress update to %d clients for user %s", sentCount, update.UserID)
}

// GetStats returns server statistics
func (s *Server) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return map[string]interface{}{
		"total_clients":    len(s.clients),
		"unique_users":     s.countUniqueUsers(),
		"broadcast_buffer": len(s.broadcast),
	}
}

// countUniqueUsers counts unique connected users
func (s *Server) countUniqueUsers() int {
	users := make(map[string]bool)
	for _, client := range s.clients {
		users[client.UserID] = true
	}
	return len(users)
}