package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket hub implementation for managing chat rooms and clients
// Workflow: cli or client -> main -> ServeWs -> Client.readPump -> Hub.broadcastToRoom -> Client.writePump

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Message represents a chat message
type Message struct {
	Type     string `json:"type"`     // "join", "leave", "chat", "system"
	Room     string `json:"room"`     // Room name
	Username string `json:"username"` // Sender's username
	Text     string `json:"text"`     // Message content
	Time     string `json:"time"`     // Timestamp HH:MM:SS
}

// Client represents a websocket client
type Client struct {
	ID       string
	Username string
	Conn     *websocket.Conn
	Room     string
	Send     chan []byte
	hub      *Hub
}

// Room represents a chat room with multiple clients
type Room struct {
	Name    string
	Clients map[*Client]bool
	History []Message
	mu      sync.RWMutex
}

// Hub maintains active clients and broadcasts messages
type Hub struct {
	rooms      map[string]*Room
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates a new Hub
func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]*Room),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.addClientToRoom(client)

		case client := <-h.unregister:
			h.removeClientFromRoom(client)
		}
	}
}

// addClientToRoom adds a client to specified room (creates room if needed)
func (h *Hub) addClientToRoom(client *Client) {
	h.mu.Lock()

	// Get existing room or create new one
	room, exists := h.rooms[client.Room]
	if !exists {
		room = &Room{
			Name:    client.Room,
			Clients: make(map[*Client]bool),
			History: make([]Message, 0, 100),
		}
		h.rooms[client.Room] = room
		log.Printf("Created new room: %s", client.Room)
	}

	// Add client to room
	room.mu.Lock()
	room.Clients[client] = true
	clientCount := len(room.Clients)
	room.mu.Unlock()

	h.mu.Unlock()

	log.Printf("Client %s joined room %s (Total: %d)",
		client.Username, client.Room, clientCount)

	// Send room history to new client
	h.sendHistoryToClient(client, room)

	// Send join notification to all clients in room
	msg := Message{
		Type:     "system",
		Room:     client.Room,
		Username: client.Username,
		Text:     fmt.Sprintf("%s joined the room", client.Username),
		Time:     time.Now().Format("15:04:05"),
	}
	h.broadcastToRoom(client.Room, msg)
}

// removeClientFromRoom removes a client from their room
func (h *Hub) removeClientFromRoom(client *Client) {
	h.mu.RLock()
	room, exists := h.rooms[client.Room]
	h.mu.RUnlock()

	if !exists {
		return
	}

	// Remove client from room
	room.mu.Lock()
	if _, ok := room.Clients[client]; ok {
		delete(room.Clients, client)
		close(client.Send)
	}
	clientCount := len(room.Clients)
	room.mu.Unlock()

	log.Printf("Client %s left room %s (Remaining: %d)",
		client.Username, client.Room, clientCount)

	// Send leave notification to remaining clients in room
	msg := Message{
		Type:     "system",
		Room:     client.Room,
		Username: client.Username,
		Text:     fmt.Sprintf("%s left the room", client.Username),
		Time:     time.Now().Format("15:04:05"),
	}
	h.broadcastToRoom(client.Room, msg)

	// Delete room if empty
	if clientCount == 0 {
		h.mu.Lock()
		delete(h.rooms, client.Room)
		h.mu.Unlock()
		log.Printf("Deleted empty room: %s", client.Room)
	}
}

// broadcastToRoom sends message to all clients in specified room only
func (h *Hub) broadcastToRoom(roomName string, msg Message) {
	h.mu.RLock()
	room, exists := h.rooms[roomName]
	h.mu.RUnlock()

	if !exists {
		return
	}

	// Add to room history (keep last 100 messages)
	room.mu.Lock()
	room.History = append(room.History, msg)
	if len(room.History) > 100 {
		room.History = room.History[1:]
	}
	room.mu.Unlock()

	// Marshal message to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}

	// Send to all clients in this room
	room.mu.RLock()
	defer room.mu.RUnlock()

	for client := range room.Clients {
		select {
		case client.Send <- data:
			// Message sent successfully
		default:
			// Channel full, client slow/dead - close it
			close(client.Send)
			delete(room.Clients, client)
		}
	}
}

// sendHistoryToClient sends recent chat history to a client
func (h *Hub) sendHistoryToClient(client *Client, room *Room) {
	room.mu.RLock()
	history := make([]Message, len(room.History))
	copy(history, room.History)
	room.mu.RUnlock()

	if len(history) == 0 {
		return
	}

	historyMsg := map[string]interface{}{
		"type":     "history",
		"messages": history,
	}
	data, err := json.Marshal(historyMsg)
	if err != nil {
		return
	}

	select {
	case client.Send <- data:
	default:
	}
}

// GetStats returns hub statistics
func (h *Hub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	totalClients := 0
	rooms := make([]map[string]interface{}, 0)

	for roomName, room := range h.rooms {
		room.mu.RLock()
		clientCount := len(room.Clients)
		totalClients += clientCount
		rooms = append(rooms, map[string]interface{}{
			"name":    roomName,
			"clients": clientCount,
		})
		room.mu.RUnlock()
	}

	return map[string]interface{}{
		"total_clients": totalClients,
		"total_rooms":   len(h.rooms),
		"rooms":         rooms,
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error: %v", err)
			}
			break
		}

		// Parse incoming JSON message
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			continue
		}

		// Set message metadata (server-side, not trusted from client)
		msg.Username = c.Username
		msg.Room = c.Room
		msg.Type = "chat"
		msg.Time = time.Now().Format("15:04:05")

		// Broadcast to room only
		c.hub.broadcastToRoom(c.Room, msg)
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWs handles websocket requests from clients
func ServeWs(hub *Hub, conn *websocket.Conn, username, room string) {
	client := &Client{
		ID:       fmt.Sprintf("%s-%d", username, time.Now().Unix()),
		Username: username,
		Room:     room,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		hub:      hub,
	}

	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}
