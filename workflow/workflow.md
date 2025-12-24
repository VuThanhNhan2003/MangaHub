# MangaHub - Workflow Documentation

This document describes the detailed workflow of different protocols and use cases in MangaHub application.

---

## 📋 Table of Contents

1. [HTTP Protocol Workflows](#http-protocol-workflows)
   - [UC-001: User Registration](#uc-001-user-registration-http)
   - [UC-002: User Authentication](#uc-002-user-authentication-http)
   - [UC-003: Search Manga](#uc-003-search-manga-http)
   - [UC-004: View Manga Details](#uc-004-view-manga-details-http)
   - [UC-005: Add Manga to Library](#uc-005-add-manga-to-library-http)
   - [UC-006: Update Reading Progress](#uc-006-update-reading-progress-http)
2. [TCP Protocol Workflows](#tcp-protocol-workflows)
   - [UC-007: Connect to TCP Sync Server](#uc-007-connect-to-tcp-sync-server-tcp)
   - [UC-008: Monitor Progress Updates](#uc-008-monitor-progress-updates-tcp)
3. [UDP Protocol Workflows](#udp-protocol-workflows)
   - [UC-009: Subscribe to UDP Notifications](#uc-009-subscribe-to-udp-notifications-udp)
   - [UC-010: Send Chapter Release Notification](#uc-010-send-chapter-release-notification-udp)
4. [WebSocket Protocol Workflows](#websocket-protocol-workflows)
   - [UC-011: Join Chat Room](#uc-011-join-chat-room-websocket)
   - [UC-012: Send Chat Message](#uc-012-send-chat-message-websocket)
   - [UC-013: Leave Chat Room](#uc-013-leave-chat-room-websocket)
5. [gRPC Protocol Workflows](#grpc-protocol-workflows)
   - [UC-014: Retrieve Manga via gRPC](#uc-014-retrieve-manga-via-grpc)
   - [UC-015: Search Manga via gRPC](#uc-015-search-manga-via-grpc)
   - [UC-016: Update Progress via gRPC](#uc-016-update-progress-via-grpc)

---

## HTTP Protocol Workflows

### UC-001: User Registration (HTTP)

**Mô tả**: Người dùng đăng ký tài khoản mới thông qua HTTP API

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleAuth() 
  → cmdAuthRegister()
    → Nhập username, email từ command flags
    → Nhập password từ stdin
    → Tạo payload JSON {username, email, password}
    → makeRequest("POST", "/auth/register", data, "")
      → Gửi HTTP POST request tới http://localhost:8080/api/auth/register
      → Nhận response JSON
    → Hiển thị kết quả: user_id, username
```

**Server Flow**:
```
cmd/server/main.go
  → Router setup: router.POST("/api/auth/register", userHandler.Register)
    → internal/user/handler.go::Register()
      → Parse và validate request body (username, email, password)
      → Gọi userService.Register(&req)
        → internal/user/handler.go::Service.Register()
          → Hash password bằng auth.HashPassword()
            → internal/auth/jwt.go::HashPassword() - Sử dụng bcrypt
          → Tạo User object với UUID mới
          → userRepo.Create(user)
            → internal/user/repository.go::Create()
              → INSERT vào bảng users trong SQLite
              → Kiểm tra duplicate username/email
      → Trả về HTTP 201 Created với user data
```

**Database Operations**:
- INSERT INTO users (id, username, email, password_hash, created_at)

---

### UC-002: User Authentication (HTTP)

**Mô tả**: Người dùng đăng nhập để lấy JWT token

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleAuth()
  → cmdAuthLogin()
    → Nhập username từ flag hoặc stdin
    → Nhập password từ stdin
    → Tạo payload {username, password}
    → makeRequest("POST", "/auth/login", data, "")
      → HTTP POST tới /api/auth/login
      → Nhận response có token
    → Lưu token vào config.User.Token
    → saveConfig() - Lưu vào ~/.mangahub/config.yaml
```

**Server Flow**:
```
cmd/server/main.go
  → Router: router.POST("/api/auth/login", userHandler.Login)
    → internal/user/handler.go::Login()
      → Parse request {username, password}
      → Gọi userService.Login(&req)
        → internal/user/handler.go::Service.Login()
          → userRepo.GetByUsername(username)
            → internal/user/repository.go::GetByUsername()
              → SELECT * FROM users WHERE username = ?
          → auth.CheckPassword(password, user.PasswordHash)
            → internal/auth/jwt.go::CheckPassword() - Verify bcrypt hash
          → auth.GenerateToken(userID, username, jwtSecret)
            → internal/auth/jwt.go::GenerateToken()
              → Tạo JWT token với claims: user_id, username, exp
              → Sign token bằng HMAC-SHA256
      → Trả về HTTP 200 OK với {token, user_id, username}
```

---

### UC-003: Search Manga (HTTP)

**Mô tả**: Tìm kiếm manga theo từ khóa

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleManga()
  → cmdMangaSearch()
    → Nhận query từ args: os.Args[3:]
    → Xây dựng URL: /manga?query=<query>
    → makeRequest("GET", url, nil, "")
      → HTTP GET tới /api/manga?query=...
    → Parse response: data.mangas[]
    → Hiển thị danh sách: ID, Title, Author, Status, Chapters
```

**Server Flow**:
```
cmd/server/main.go
  → Router: router.GET("/api/manga", mangaHandler.SearchManga)
    → internal/manga/handler.go::SearchManga()
      → Parse query parameters: query, genre, status, limit, page
      → Validate và set defaults (limit=20, page=1)
      → Tính offset = (page - 1) * limit
      → mangaRepo.Search(query, genre, status, limit, offset)
        → internal/manga/repository.go::Search()
          → Xây dựng SQL query động
          → SELECT * FROM manga WHERE 
              title LIKE ? OR author LIKE ?
              AND genre LIKE ? (nếu có)
              AND status = ? (nếu có)
              LIMIT ? OFFSET ?
          → Scan kết quả vào []models.Manga
      → Trả về HTTP 200 với {mangas, page, limit, count}
```

---

### UC-004: View Manga Details (HTTP)

**Mô tả**: Xem chi tiết thông tin manga

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleManga()
  → cmdMangaInfo()
    → Nhận manga_id từ args[3]
    → makeRequest("GET", "/manga/"+mangaID, nil, config.User.Token)
      → HTTP GET tới /api/manga/:id
      → Có thể kèm Authorization header nếu đã login
    → Parse response: data.manga, data.progress
    → Hiển thị: Title, Author, Status, Chapters, Description
    → Nếu có progress: Status, Current Chapter, Rating
```

**Server Flow**:
```
cmd/server/main.go
  → Router: router.GET("/api/manga/:id", mangaHandler.GetManga)
    → internal/manga/handler.go::GetManga()
      → Lấy mangaID từ path parameter: c.Param("id")
      → mangaRepo.GetByID(mangaID)
        → internal/manga/repository.go::GetByID()
          → SELECT * FROM manga WHERE id = ?
          → Trả về models.Manga hoặc ErrMangaNotFound
      → Nếu có JWT token:
        → auth.GetUserID(c)
          → internal/auth/middleware.go::GetUserID()
            → Extract user_id từ gin.Context
        → mangaRepo.GetProgress(userID, mangaID)
          → internal/manga/repository.go::GetProgress()
            → SELECT * FROM user_progress WHERE user_id=? AND manga_id=?
      → Trả về HTTP 200 với {manga, progress}
```

---

### UC-005: Add Manga to Library (HTTP)

**Mô tả**: Thêm manga vào thư viện cá nhân

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleLibrary()
  → cmdLibraryAdd()
    → Kiểm tra authentication: requireAuth()
    → Lấy flags: --manga-id, --status
    → Tạo payload: {manga_id, status, current_chapter: 0, rating: 0}
    → makeRequest("POST", "/library", data, config.User.Token)
      → HTTP POST tới /api/library với Authorization header
    → Hiển thị thành công
```

**Server Flow**:
```
cmd/server/main.go
  → Router: protected.POST("/api/library", mangaHandler.AddToLibrary)
    → Middleware: auth.JWTMiddleware(jwtSecret)
      → internal/auth/middleware.go::JWTMiddleware()
        → Parse Authorization header: "Bearer <token>"
        → Validate JWT token
        → Extract claims và set vào gin.Context
    → internal/manga/handler.go::AddToLibrary()
      → auth.GetUserID(c) - Lấy user_id từ context
      → Parse request: {manga_id, status, current_chapter, rating}
      → Validate status (reading, completed, plan-to-read, on-hold, dropped)
      → Validate rating (0-10)
      → mangaRepo.GetByID(manga_id) - Kiểm tra manga tồn tại
      → Tạo models.UserProgress object
      → mangaRepo.AddToLibrary(progress)
        → internal/manga/repository.go::AddToLibrary()
          → INSERT INTO user_progress (user_id, manga_id, current_chapter, 
              status, rating, started_at, updated_at)
      → Gửi UDP notification (nếu udpServer != nil):
        → udpServer.SendNotificationToUser(userID, notification)
          → internal/udp/server.go::SendNotificationToUser()
      → Trả về HTTP 201 Created
```

---

### UC-006: Update Reading Progress (HTTP)

**Mô tả**: Cập nhật tiến trình đọc và broadcast tới TCP clients

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleProgress()
  → cmdProgressUpdate()
    → requireAuth() - Kiểm tra đã login
    → Lấy flags: --manga-id, --chapter
    → Tạo payload: {manga_id, chapter}
    → makeRequest("PUT", "/progress", data, config.User.Token)
      → HTTP PUT tới /api/progress
    → Hiển thị kết quả: manga_title, chapter
```

**Server Flow**:
```
cmd/server/main.go
  → Router: protected.PUT("/api/progress", mangaHandler.UpdateProgress)
    → Middleware: auth.JWTMiddleware(jwtSecret)
    → internal/manga/handler.go::UpdateProgress()
      → auth.GetUserID(c)
      → Parse request: {manga_id, chapter}
      → Validate chapter >= 0
      → mangaRepo.GetByID(manga_id) - Validate manga exists
      → Kiểm tra chapter <= total_chapters
      → mangaRepo.UpdateProgress(userID, mangaID, chapter)
        → internal/manga/repository.go::UpdateProgress()
          → UPDATE user_progress 
              SET current_chapter = ?, updated_at = ?
              WHERE user_id = ? AND manga_id = ?
          → Nếu chapter == total_chapters:
              → Tự động cập nhật status = "completed"
      → Broadcast via TCP (non-blocking):
        → Tạo models.ProgressUpdate
        → progressBroadcast <- update
          → Channel được connect tới TCP server
          → tcpServer.GetBroadcastChannel() <- update
            → internal/tcp/server.go::handleBroadcasts()
              → Gửi tới tất cả TCP clients của user
      → Gửi UDP notification:
        → udpServer.SendNotificationToUser(userID, notification)
      → Trả về HTTP 200 với {manga_id, chapter, manga_title}
```

**Cross-Protocol Interaction**:
- HTTP request trigger → TCP broadcast → All connected TCP clients nhận update
- HTTP request trigger → UDP notification → User's UDP client nhận thông báo

---

## TCP Protocol Workflows

### UC-007: Connect to TCP Sync Server (TCP)

**Mô tả**: Kết nối tới TCP server để nhận real-time updates

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleSync()
  → cmdSyncConnect()
    → net.Dial("tcp", "localhost:9090")
      → Establish TCP connection
    → Tạo auth message: {user_id: config.User.UserID}
    → Marshal to JSON và gửi: conn.Write(authData + '\n')
    → bufio.NewReader(conn).ReadBytes('\n')
      → Đọc confirmation response
    → Parse JSON response: {status, message, client_id}
    → Hiển thị connection established
```

**Server Flow**:
```
cmd/server/main.go
  → tcpServer.Start()
    → internal/tcp/server.go::Start()
      → net.Listen("tcp", ":9090")
      → Chạy goroutine handleBroadcasts() - Lắng nghe channel
      → Loop accept connections:
        → listener.Accept()
        → Spawn goroutine: handleConnection(conn)
          → internal/tcp/server.go::handleConnection()
            → bufio.NewReader(conn).ReadBytes('\n')
              → Đọc authentication message
            → Parse JSON: {user_id}
            → Validate user_id not empty
            → Tạo Client object: {Conn, UserID}
            → Generate unique clientID: userID_timestamp
            → s.mutex.Lock()
            → s.clients[clientID] = client
            → s.mutex.Unlock()
            → Gửi confirmation: {status: "connected", message, client_id}
            → Start heartbeat handler loop:
              → conn.SetReadDeadline(30s intervals)
              → Đọc messages từ client
              → Nếu nhận "heartbeat": gửi "heartbeat_ack"
```

**Connection Lifecycle**:
1. TCP handshake
2. Client gửi auth message
3. Server validate và register client
4. Server gửi confirmation
5. Heartbeat mechanism duy trì connection

---

### UC-008: Monitor Progress Updates (TCP)

**Mô tả**: Lắng nghe và hiển thị real-time progress updates

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleSync()
  → cmdSyncMonitor()
    → Establish connection như UC-007
    → Gửi authentication và nhận confirmation
    → Setup signal handler (Ctrl+C): os.Signal channel
    → Spawn goroutine: Heartbeat sender
      → time.NewTicker(30 * time.Second)
      → Mỗi 30s: conn.Write({type: "heartbeat"})
    → Main loop: reader.ReadBytes('\n')
      → Parse JSON message từ server
      → Nếu type == "progress_update":
        → Hiển thị: timestamp, manga_id, chapter
      → Nếu type == "heartbeat_ack": im lặng
    → Khi nhận SIGINT:
      → Đóng connection và exit
```

**Server Flow**:
```
cmd/server/main.go::main()
  → progressBroadcast := make(chan models.ProgressUpdate, 100)
  → Connect channel to TCP server:
    → go func() {
        for update := range progressBroadcast {
          tcpServer.GetBroadcastChannel() <- update
        }
      }()
  
internal/tcp/server.go::handleBroadcasts()
  → Goroutine chạy background
  → Loop: for update := range s.broadcast
    → Parse ProgressUpdate: {UserID, MangaID, Chapter, Timestamp}
    → s.mutex.RLock()
    → Tìm tất cả clients có UserID trùng khớp
    → s.mutex.RUnlock()
    → Với mỗi matching client:
      → Tạo JSON message: {type: "progress_update", ...}
      → Marshal to JSON
      → client.Conn.Write(jsonData + '\n')
      → Nếu write error: Log và remove client
```

**Broadcast Trigger**:
```
HTTP UpdateProgress endpoint (UC-006)
  → progressBroadcast <- models.ProgressUpdate
    → TCP server handleBroadcasts() nhận message
      → Broadcast tới all matching TCP clients
```

---

## UDP Protocol Workflows

### UC-009: Subscribe to UDP Notifications (UDP)

**Mô tả**: Đăng ký nhận notification qua UDP

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleNotify()
  → cmdNotifySubscribe()
    → net.ResolveUDPAddr("udp", "localhost:9091")
    → net.DialUDP("udp", nil, serverAddr)
    → Tạo registration message:
      → {type: "register", user_id, preferences: {chapter_releases, system_updates}}
    → Marshal to JSON
    → conn.Write(jsonData) - Gửi UDP packet
    → conn.SetReadDeadline(5s)
    → conn.ReadFromUDP(buffer) - Đợi confirmation
    → Parse response: {status: "registered", message, preferences}
    → Hiển thị subscription successful
    → Setup signal handler (SIGINT):
      → Gửi {type: "unregister", user_id}
    → Spawn goroutine: Keep-alive sender
      → time.NewTicker(30s)
      → Gửi {type: "ping"}
    → Main loop: conn.ReadFromUDP(buffer)
      → Parse notification messages
      → Hiển thị: timestamp, title, manga_title, chapter
```

**Server Flow**:
```
cmd/server/main.go
  → udpServer.Start()
    → internal/udp/server.go::Start()
      → net.ResolveUDPAddr("udp", ":9091")
      → net.ListenUDP("udp", addr)
      → Spawn goroutine: cleanupInactiveClients()
        → Mỗi 1 phút: xóa clients inactive > 5 phút
      → Main loop: conn.ReadFromUDP(buffer)
        → Spawn goroutine: handleMessage(data, clientAddr)
          → Parse JSON message
          → Switch msgType:
            
            CASE "register":
              → handleRegister(msg, addr)
                → Extract user_id, preferences
                → s.mutex.Lock()
                → s.clients[addr.String()] = &UDPClient{
                    Addr, UserID, LastSeen: now, Preferences
                  }
                → s.mutex.Unlock()
                → sendToClient(addr, confirmation)
            
            CASE "unregister":
              → handleUnregister(msg, addr)
                → s.mutex.Lock()
                → delete(s.clients, addr.String())
                → s.mutex.Unlock()
                → sendToClient(addr, confirmation)
            
            CASE "ping":
              → handlePing(addr)
                → Update LastSeen
                → sendToClient(addr, {type: "pong"})
```

---

### UC-010: Send Chapter Release Notification (UDP)

**Mô tả**: Admin gửi notification về chapter mới

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleNotify()
  → cmdNotifySend()
    → requireAuth()
    → Lấy flags: --manga-id, --chapter
    → makeRequest("POST", "/notify/chapter", data, config.User.Token)
      → HTTP POST tới /api/notify/chapter
    → Hiển thị: notification sent, manga_title, chapter
```

**Server Flow**:
```
cmd/server/main.go
  → Router: protected.POST("/api/notify/chapter", mangaHandler.SendNotification)
    → Middleware: auth.JWTMiddleware
    → internal/manga/handler.go::SendNotification()
      → Parse request: {manga_id, chapter}
      → mangaRepo.GetByID(manga_id) - Lấy manga details
      → udpServer.SendChapterNotification(manga.Title, chapter, mangaID)
        → internal/udp/server.go::SendChapterNotification()
          → Tạo notification message:
            → {type: "notification", title: "New Chapter Released",
                manga_title, chapter, timestamp}
          → s.mutex.RLock()
          → Lặp qua tất cả s.clients:
            → Kiểm tra preferences["chapter_releases"] == true
            → Marshal to JSON
            → s.conn.WriteToUDP(jsonData, client.Addr)
          → s.mutex.RUnlock()
      → Trả về HTTP 200 với notification details
```

**UDP Notification Flow**:
```
HTTP API request (admin)
  → mangaHandler.SendNotification()
    → udpServer.SendChapterNotification()
      → Broadcast UDP packets tới all registered clients
        → Clients listening on cmdNotifySubscribe() nhận packets
          → Display notification
```

---

## WebSocket Protocol Workflows

### UC-011: Join Chat Room (WebSocket)

**Mô tả**: Kết nối tới chat room qua WebSocket

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleChat()
  → cmdChatJoin()
    → Lấy room name từ args (default: "general")
    → Lấy username từ config hoặc stdin
    → Xây dựng WebSocket URL:
      → ws://localhost:8080/ws?username=<name>&room=<room>
    → websocket.DefaultDialer.Dial(wsURL, nil)
      → WebSocket handshake
    → Setup signal handler (SIGINT)
    → Spawn goroutine: Message reader
      → Loop: conn.ReadMessage()
        → Parse JSON message
        → Nếu type == "history":
          → Hiển thị recent chat history
        → Nếu type == "chat" hoặc "system":
          → displayMessage(msg) - Hiển thị với format
    → Spawn goroutine: Message sender
      → bufio.Scanner đọc stdin
      → Parse commands: /quit, /help
      → Tạo message: {text}
      → Marshal to JSON
      → conn.WriteMessage(websocket.TextMessage, jsonData)
    → Select wait: done channel hoặc interrupt signal
```

**Server Flow**:
```
cmd/server/main.go
  → Router: router.GET("/ws", func(c *gin.Context) {
      username := c.Query("username")
      room := c.Query("room")
      → upgrader.Upgrade(c.Writer, c.Request, nil)
        → HTTP -> WebSocket upgrade
      → internal/websocket/hub.go::ServeWs(chatHub, conn, username, room)
    })

internal/websocket/hub.go::ServeWs()
  → Tạo Client object: {ID, Username, Conn, Room, Send: buffered channel}
  → client.hub = hub
  → hub.register <- client - Gửi vào register channel
  → Spawn goroutine: client.writePump()
    → Loop: select từ client.Send channel
      → conn.SetWriteDeadline()
      → conn.WriteMessage(websocket.TextMessage, message)
      → Gửi periodic ping messages
  → client.readPump() (blocking)
    → conn.SetReadDeadline()
    → conn.SetPongHandler() - Handle pong responses
    → Loop: conn.ReadMessage()
      → Parse JSON message
      → Nếu có text:
        → Tạo Message: {Type: "chat", Room, Username, Text, Time}
        → hub.broadcastToRoom(room, message)

Hub.Run() goroutine:
  → Loop select:
    → CASE client := <-h.register:
      → addClientToRoom(client)
        → Tạo room nếu chưa tồn tại
        → room.Clients[client] = true
        → sendHistoryToClient(client, room) - Gửi history
        → broadcastToRoom(room, {type: "system", text: "joined"})
    
    → CASE client := <-h.unregister:
      → removeClientFromRoom(client)
        → delete(room.Clients, client)
        → close(client.Send)
        → broadcastToRoom(room, {type: "system", text: "left"})

broadcastToRoom(room, message):
  → room.mu.RLock()
  → Lặp qua room.Clients:
    → client.Send <- marshaledJSON (non-blocking)
  → room.mu.RUnlock()
  → Append message to room.History (giới hạn 100)
```

---

### UC-012: Send Chat Message (WebSocket)

**Mô tả**: Gửi message trong chat room

**Client Flow**:
```
cmdChatJoin() message sender goroutine:
  → scanner.Scan() - Đọc từ stdin
  → strings.TrimSpace(text)
  → Nếu text == "/quit": Gửi CloseMessage và exit
  → Nếu text == "/help": Hiển thị help và continue
  → Tạo message: {text: text}
  → json.Marshal(msg)
  → conn.WriteMessage(websocket.TextMessage, jsonData)
    → Gửi qua WebSocket connection
```

**Server Flow**:
```
internal/websocket/hub.go::Client.readPump()
  → conn.ReadMessage() - Nhận message từ client
  → json.Unmarshal(data, &msg)
  → Parse msg["text"]
  → Tạo Message object:
    → {Type: "chat", Room: c.Room, Username: c.Username, 
       Text: text, Time: "HH:MM:SS"}
  → c.hub.broadcastToRoom(c.Room, message)
    → internal/websocket/hub.go::broadcastToRoom()
      → h.mu.RLock()
      → room := h.rooms[roomName]
      → h.mu.RUnlock()
      → room.mu.RLock()
      → Lặp qua room.Clients:
        → Marshal message to JSON
        → select { case client.Send <- jsonData: ... }
          → Non-blocking send
      → room.mu.RUnlock()
      → Append to room.History
```

**Message Flow**:
```
Client A stdin input
  → conn.WriteMessage()
    → Server Client.readPump() nhận
      → hub.broadcastToRoom()
        → Gửi vào client.Send channel của tất cả clients trong room
          → Client.writePump() của mỗi client
            → conn.WriteMessage() gửi lại về clients
              → Client B, C, D... readPump() nhận
                → displayMessage() hiển thị
```

---

### UC-013: Leave Chat Room (WebSocket)

**Mô tả**: Ngắt kết nối khỏi chat room

**Client Flow**:
```
cmdChatJoin():
  → Khi user type "/quit" hoặc nhấn Ctrl+C:
    → conn.WriteMessage(websocket.CloseMessage, 
        websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
    → os.Exit(0) hoặc return
  
  → Hoặc connection error trong readPump goroutine:
    → log.Println("Connection closed:", err)
    → close(done) channel
    → Trigger cleanup
```

**Server Flow**:
```
internal/websocket/hub.go::Client.readPump()
  → conn.ReadMessage() returns error:
    → Có thể là CloseMessage hoặc network error
    → Break loop
    → defer cleanup:
      → c.hub.unregister <- c - Gửi vào unregister channel
      → c.Conn.Close()

Hub.Run() goroutine:
  → CASE client := <-h.unregister:
    → removeClientFromRoom(client)
      → h.mu.RLock()
      → room := h.rooms[client.Room]
      → h.mu.RUnlock()
      → room.mu.Lock()
      → if _, ok := room.Clients[client]; ok:
        → delete(room.Clients, client)
        → close(client.Send) - Đóng send channel
      → clientCount := len(room.Clients)
      → room.mu.Unlock()
      → Log: "Client left room"
      → broadcastToRoom(room, {type: "system", text: "username left"})
```

**Cleanup Flow**:
```
Client disconnect (close/error)
  → readPump() detects error
    → hub.unregister <- client
      → Hub.Run() processes unregister
        → removeClientFromRoom()
          → Remove from room.Clients map
          → Close client.Send channel
          → Broadcast "left" message to remaining clients
        → writePump() goroutine receives closed channel
          → Exits goroutine
```

---

## gRPC Protocol Workflows

### UC-014: Retrieve Manga via gRPC

**Mô tả**: Lấy thông tin manga qua gRPC

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleGRPC()
  → cmdGRPCGet()
    → Lấy manga_id từ flags: --manga-id
    → grpc.NewClient("localhost:9092", insecure credentials)
      → Establish gRPC connection
    → pb.NewMangaServiceClient(conn)
    → context.WithTimeout(5 seconds)
    → client.GetManga(ctx, &pb.GetMangaRequest{MangaId: mangaID})
      → Gọi gRPC method
    → Nhận pb.MangaResponse
    → Hiển thị: Title, Author, Status, Chapters, Year, Genres, Description
    → conn.Close()
```

**Server Flow**:
```
cmd/server/main.go
  → Spawn goroutine: Start gRPC server
    → net.Listen("tcp", ":9092")
    → grpcSrv := grpc.NewServer()
    → server := grpcServer.NewServer(mangaRepo)
    → pb.RegisterMangaServiceServer(grpcSrv, server)
    → grpcSrv.Serve(lis)

internal/grpc/server.go::GetManga()
  → Nhận pb.GetMangaRequest với MangaId
  → log.Printf("gRPC GetManga called for ID: %s", req.MangaId)
  → s.repo.GetByID(req.MangaId)
    → internal/manga/repository.go::GetByID()
      → SELECT * FROM manga WHERE id = ?
      → Scan vào models.Manga
  → Nếu not found: return status.Error(codes.NotFound, ...)
  → Parse genres từ JSON string
  → Construct pb.MangaResponse:
    → {Id, Title, Author, Genres, Status, TotalChapters, 
       Description, CoverUrl, Year}
  → Return response
```

**Protocol Details**:
- Transport: HTTP/2
- Serialization: Protocol Buffers
- Request: pb.GetMangaRequest protobuf message
- Response: pb.MangaResponse protobuf message

---

### UC-015: Search Manga via gRPC

**Mô tả**: Tìm kiếm manga qua gRPC

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleGRPC()
  → cmdGRPCSearch()
    → Parse query từ flags: --query
    → grpc.NewClient("localhost:9092")
    → pb.NewMangaServiceClient(conn)
    → context.WithTimeout(5s)
    → client.SearchManga(ctx, &pb.SearchRequest{
        Query: query, Limit: 10, Offset: 0
      })
    → Nhận pb.SearchResponse với Mangas array
    → Loop through resp.Mangas:
      → Hiển thị: Index, Title, ID, Author, Status, Chapters
    → conn.Close()
```

**Server Flow**:
```
internal/grpc/server.go::SearchManga()
  → Nhận pb.SearchRequest: {Query, Genre, Status, Limit, Offset}
  → log.Printf("gRPC SearchManga called with query: %s", req.Query)
  → Validate limit (default 20 nếu <= 0)
  → s.repo.Search(query, genre, status, limit, offset)
    → internal/manga/repository.go::Search()
      → Build dynamic SQL query với LIKE patterns
      → SELECT với filters và pagination
      → Scan vào []models.Manga
  → Loop through mangas:
    → Parse genres từ JSON
    → Create pb.MangaResponse cho mỗi manga
    → Append vào results array
  → Return pb.SearchResponse{Mangas: results, TotalCount: len(results)}
```

---

### UC-016: Update Progress via gRPC

**Mô tả**: Cập nhật progress và trigger TCP broadcast qua gRPC

**Client Flow (CLI)**:
```
cmd/cli/main.go::handleGRPC()
  → cmdGRPCUpdate()
    → requireAuth()
    → Lấy flags: --manga-id, --chapter
    → grpc.NewClient("localhost:9092")
    → pb.NewMangaServiceClient(conn)
    → context.WithTimeout(5s)
    → client.UpdateProgress(ctx, &pb.UpdateProgressRequest{
        UserId: config.User.UserID,
        MangaId: mangaID,
        Chapter: chapterNum
      })
    → Nhận pb.UpdateProgressResponse
    → Nếu Success: Hiển thị chapter và message
    → conn.Close()
```

**Server Flow**:
```
internal/grpc/server.go::UpdateProgress()
  → Nhận pb.UpdateProgressRequest: {UserId, MangaId, Chapter}
  → log.Printf("gRPC UpdateProgress for user %s, manga %s, chapter %d")
  → s.repo.GetByID(req.MangaId)
    → Validate manga exists
  → Validate chapter <= total_chapters
  → s.repo.UpdateProgress(req.UserId, req.MangaId, int(req.Chapter))
    → internal/manga/repository.go::UpdateProgress()
      → UPDATE user_progress SET current_chapter=?, updated_at=?
      → WHERE user_id=? AND manga_id=?
  → Nếu error: Return pb.UpdateProgressResponse{Success: false, Message}
  → Return pb.UpdateProgressResponse{
      Success: true,
      Message: "progress updated successfully",
      CurrentChapter: req.Chapter,
      UpdatedAt: time.Now().Unix()
    }
```

**Note**: gRPC UpdateProgress hiện tại chưa trigger TCP broadcast như HTTP endpoint. Nếu cần, phải thêm logic:
```
Sau khi UpdateProgress thành công:
  → Tạo models.ProgressUpdate
  → Gửi vào progressBroadcast channel
  → TCP server nhận và broadcast
```

---

## Cross-Protocol Integration

### HTTP → TCP Broadcast
```
UC-006: HTTP UpdateProgress
  → mangaHandler.UpdateProgress()
    → progressBroadcast <- update (channel)
      → tcpServer.handleBroadcasts() (goroutine)
        → Broadcast tới all TCP clients của user
```

### HTTP → UDP Notification
```
UC-005: HTTP AddToLibrary
  → mangaHandler.AddToLibrary()
    → udpServer.SendNotificationToUser(userID, notification)
      → Gửi UDP packet tới registered client

UC-010: HTTP SendNotification (admin)
  → mangaHandler.SendNotification()
    → udpServer.SendChapterNotification()
      → Broadcast UDP tới all registered clients
```

### Multi-Protocol Flow Example
```
User updates progress via HTTP:
  1. CLI: makeRequest("PUT", "/progress") [HTTP]
  2. Server: mangaHandler.UpdateProgress()
     - Database: UPDATE user_progress [SQLite]
     - Channel: progressBroadcast <- update [Go channel]
     - UDP: SendNotificationToUser() [UDP packet]
  3. TCP Server: handleBroadcasts() nhận từ channel
     - Broadcast tới all TCP clients [TCP packets]
  4. CLI monitoring: cmdSyncMonitor() nhận update [TCP]
     - Display: "Progress Update: Chapter 15"
  5. CLI subscribed: cmdNotifySubscribe() nhận notification [UDP]
     - Display: "Updated progress to chapter 15"
```

---

## Architecture Summary

### Server Components
```
main.go
  ├─ HTTP Server (Gin) :8080
  │   ├─ Public routes (/auth/*, /manga)
  │   ├─ Protected routes (JWT middleware)
  │   └─ WebSocket upgrade (/ws)
  │
  ├─ TCP Server :9090
  │   ├─ Accept connections
  │   ├─ handleConnection() goroutines
  │   └─ handleBroadcasts() goroutine
  │
  ├─ UDP Server :9091
  │   ├─ ReadFromUDP() loop
  │   ├─ handleMessage() goroutines
  │   └─ cleanupInactiveClients() goroutine
  │
  ├─ gRPC Server :9092
  │   └─ pb.MangaServiceServer implementation
  │
  └─ WebSocket Hub
      ├─ Run() goroutine (register/unregister)
      ├─ Multiple Rooms
      └─ Client readPump/writePump goroutines
```

### Client (CLI) Operations
```
cmd/cli/main.go
  ├─ HTTP requests: makeRequest()
  ├─ TCP connections: net.Dial("tcp")
  ├─ UDP connections: net.DialUDP()
  ├─ gRPC calls: grpc.NewClient()
  └─ WebSocket: websocket.DefaultDialer.Dial()
```

### Internal Modules
```
internal/
  ├─ auth/          - JWT generation/validation, bcrypt hashing
  ├─ user/          - User CRUD, authentication service
  ├─ manga/         - Manga CRUD, library management
  ├─ tcp/           - TCP server, client management, broadcasts
  ├─ udp/           - UDP server, notifications, client registry
  ├─ websocket/     - WebSocket hub, rooms, message routing
  └─ grpc/          - gRPC service implementation
```

---

## Conclusion

Workflow này miêu tả chi tiết luồng xử lý của từng Use Case qua các protocol khác nhau:
- **HTTP**: RESTful API cho CRUD operations, authentication
- **TCP**: Persistent connections cho real-time progress sync
- **UDP**: Connectionless notifications cho chapter releases
- **WebSocket**: Bidirectional communication cho chat system
- **gRPC**: High-performance RPC cho internal services

Mỗi protocol được tối ưu cho use case cụ thể, và chúng tương tác với nhau thông qua channels, shared data structures và database.
