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
	shutdown  chan struct{}
	wg        sync.WaitGroup
}

func NewServer(port string) *Server {
	return &Server{
		port:      port,
		clients:   make(map[string]*Client),
		broadcast: make(chan models.ProgressUpdate, 100),
		shutdown:  make(chan struct{}),
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
	s.wg.Add(1)
	go s.handleBroadcasts()

	// Accept connections
	go func() {
		for {
			select {
			case <-s.shutdown:
				listener.Close()
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					select {
					case <-s.shutdown:
						return
					default:
						log.Printf("Error accepting connection: %v", err)
						continue
					}
				}

				s.wg.Add(1)
				go s.handleConnection(conn)
			}
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() {
	close(s.shutdown)
	
	// Close all client connections
	s.mutex.Lock()
	for _, client := range s.clients {
		client.Conn.Close()
	}
	s.mutex.Unlock()
	
	s.wg.Wait()
	log.Println("TCP server shut down gracefully")
}

// GetBroadcastChannel returns the broadcast channel
func (s *Server) GetBroadcastChannel() chan models.ProgressUpdate {
	return s.broadcast
}

// handleConnection handles individual client connections
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	reader := bufio.NewReader(conn)
	
	// Read authentication message
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
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

	if authMsg.UserID == "" {
		log.Printf("Empty user_id in auth message")
		return
	}

	// Register client with proper locking
	clientID := fmt.Sprintf("%s_%d", authMsg.UserID, time.Now().UnixNano())
	client := &Client{
		Conn:   conn,
		UserID: authMsg.UserID,
	}

	s.mutex.Lock()
	s.clients[clientID] = client
	clientCount := len(s.clients)
	s.mutex.Unlock()

	log.Printf("Client connected: %s (UserID: %s) - Total clients: %d", clientID, authMsg.UserID, clientCount)

	// Send confirmation
	response := map[string]interface{}{
		"status":  "connected",
		"message": "Successfully connected to TCP sync server",
		"client_id": clientID,
	}
	respData, _ := json.Marshal(response)
	conn.Write(append(respData, '\n'))

	// Keep connection alive and handle heartbeats
	for {
		select {
		case <-s.shutdown:
			return
		default:
			// Set read deadline
			conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
			
			data, err := reader.ReadBytes('\n')
			if err != nil {
				goto cleanup
			}

			// Handle heartbeat or other messages
			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err == nil {
				if msgType, ok := msg["type"].(string); ok && msgType == "heartbeat" {
					// Send heartbeat response
					hbResp := map[string]string{
						"type": "heartbeat_ack",
						"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
					}
					hbData, _ := json.Marshal(hbResp)
					conn.Write(append(hbData, '\n'))
				}
			}
		}
	}

cleanup:
	// Remove client on disconnect with proper locking
	s.mutex.Lock()
	delete(s.clients, clientID)
	remainingClients := len(s.clients)
	s.mutex.Unlock()

	log.Printf("Client disconnected: %s - Remaining clients: %d", clientID, remainingClients)
}

// handleBroadcasts listens for progress updates and broadcasts them
func (s *Server) handleBroadcasts() {
	defer s.wg.Done()
	
	for {
		select {
		case <-s.shutdown:
			return
		case update := <-s.broadcast:
			s.broadcastUpdate(update)
		}
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
	failedCount := 0
	
	for clientID, client := range s.clients {
		if client.UserID == update.UserID {
			client.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := client.Conn.Write(data)
			if err != nil {
				log.Printf("Error sending to client %s: %v", clientID, err)
				failedCount++
			} else {
				sentCount++
			}
		}
	}

	log.Printf("Broadcasted progress update for user %s (manga: %s, ch: %d) - Sent: %d, Failed: %d", 
		update.UserID, update.MangaID, update.Chapter, sentCount, failedCount)
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