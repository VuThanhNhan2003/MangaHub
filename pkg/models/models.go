package models

import "time"

// User represents a registered user
type User struct {
	ID           string    `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// Manga represents a manga series
type Manga struct {
	ID            string   `json:"id" db:"id"`
	Title         string   `json:"title" db:"title"`
	Author        string   `json:"author" db:"author"`
	Genres        string   `json:"genres" db:"genres"` // JSON array as text
	Status        string   `json:"status" db:"status"` // ongoing, completed
	TotalChapters int      `json:"total_chapters" db:"total_chapters"`
	Description   string   `json:"description" db:"description"`
	CoverURL      string   `json:"cover_url" db:"cover_url"`
	Year          int      `json:"year" db:"year"`
}

// UserProgress represents user's reading progress
type UserProgress struct {
	UserID         string    `json:"user_id" db:"user_id"`
	MangaID        string    `json:"manga_id" db:"manga_id"`
	CurrentChapter int       `json:"current_chapter" db:"current_chapter"`
	Status         string    `json:"status" db:"status"` // reading, completed, plan-to-read, on-hold, dropped
	Rating         int       `json:"rating" db:"rating"` // 1-10, 0 means unrated
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
	StartedAt      time.Time `json:"started_at" db:"started_at"`
}

// ProgressUpdate represents a progress update event
type ProgressUpdate struct {
	UserID    string `json:"user_id"`
	MangaID   string `json:"manga_id"`
	Chapter   int    `json:"chapter"`
	Timestamp int64  `json:"timestamp"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// Notification represents a notification message
type Notification struct {
	Type      string `json:"type"`
	MangaID   string `json:"manga_id"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// LoginRequest represents login credentials
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest represents registration data
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=30"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// SearchRequest represents manga search parameters
type SearchRequest struct {
	Query  string `form:"query"`
	Genre  string `form:"genre"`
	Status string `form:"status"`
	Limit  int    `form:"limit" `
	Page   int    `form:"page" `
}

// AddToLibraryRequest represents request to add manga to library
type AddToLibraryRequest struct {
	MangaID        string `json:"manga_id" binding:"required"`
	Status         string `json:"status" binding:"required"`
	CurrentChapter int    `json:"current_chapter"`
	Rating         int    `json:"rating" binding:"min=0,max=10"`
}

// UpdateProgressRequest represents progress update request
type UpdateProgressRequest struct {
	MangaID string `json:"manga_id" binding:"required"`
	Chapter int    `json:"chapter" binding:"required,min=1"`
}

// Response represents standard API response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}