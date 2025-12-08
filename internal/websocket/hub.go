package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"mangahub/pkg/models"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Client represents a websocket client
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userID   string
	username string
}

// Hub maintains active clients and broadcasts messages
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	history    []models.ChatMessage
}

// NewHub creates a new Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		history:    make([]models.ChatMessage, 0, 100),
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Printf("WebSocket client connected: %s", client.username)
			
			// Send recent history to new client
			h.sendHistoryToClient(client)
			
			// Broadcast join message
			joinMsg := models.ChatMessage{
				UserID:    "system",
				Username:  "System",
				Message:   client.username + " joined the chat",
				Timestamp: time.Now().Unix(),
			}
			h.broadcastMessage(joinMsg)

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Printf("WebSocket client disconnected: %s", client.username)
				
				// Broadcast leave message
				leaveMsg := models.ChatMessage{
					UserID:    "system",
					Username:  "System",
					Message:   client.username + " left the chat",
					Timestamp: time.Now().Unix(),
				}
				h.broadcastMessage(leaveMsg)
			}

		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// broadcastMessage broadcasts a chat message to all clients
func (h *Hub) broadcastMessage(msg models.ChatMessage) {
	// Add to history (keep last 100 messages)
	h.history = append(h.history, msg)
	if len(h.history) > 100 {
		h.history = h.history[1:]
	}

	// Broadcast to all clients
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}
	h.broadcast <- data
}

// sendHistoryToClient sends recent chat history to a client
func (h *Hub) sendHistoryToClient(client *Client) {
	historyMsg := map[string]interface{}{
		"type":    "history",
		"messages": h.history,
	}
	data, err := json.Marshal(historyMsg)
	if err != nil {
		return
	}
	
	select {
	case client.send <- data:
	default:
	}
}

// GetStats returns hub statistics
func (h *Hub) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"active_clients": len(h.clients),
		"message_history": len(h.history),
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Parse message
		var incomingMsg map[string]interface{}
		if err := json.Unmarshal(message, &incomingMsg); err != nil {
			continue
		}

		// Handle different message types
		msgType, ok := incomingMsg["type"].(string)
		if !ok {
			continue
		}

		switch msgType {
		case "chat":
			text, ok := incomingMsg["message"].(string)
			if !ok || text == "" {
				continue
			}

			chatMsg := models.ChatMessage{
				UserID:    c.userID,
				Username:  c.username,
				Message:   text,
				Timestamp: time.Now().Unix(),
			}

			c.hub.broadcastMessage(chatMsg)
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWs handles websocket requests from clients
func ServeWs(hub *Hub, conn *websocket.Conn, userID, username string) {
	client := &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		userID:   userID,
		username: username,
	}

	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}