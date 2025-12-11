package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

// InitDB initializes the database connection and creates tables
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create tables
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	log.Println("Database initialized successfully")
	return db, nil
}

func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS manga (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		author TEXT NOT NULL,
		genres TEXT NOT NULL,
		status TEXT NOT NULL,
		total_chapters INTEGER NOT NULL,
		description TEXT,
		cover_url TEXT,
		manga_url TEXT,
		year INTEGER
	);

	CREATE TABLE IF NOT EXISTS user_progress (
		user_id TEXT NOT NULL,
		manga_id TEXT NOT NULL,
		current_chapter INTEGER DEFAULT 0,
		status TEXT NOT NULL,
		rating INTEGER DEFAULT 0,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, manga_id),
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_manga_title ON manga(title);
	CREATE INDEX IF NOT EXISTS idx_manga_author ON manga(author);
	CREATE INDEX IF NOT EXISTS idx_user_progress_user ON user_progress(user_id);
	CREATE INDEX IF NOT EXISTS idx_user_progress_manga ON user_progress(manga_id);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}

// SeedData seeds initial manga data
func SeedData(db *sql.DB) error {
	// Check if data already exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM manga").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		log.Println("Database already contains data, skipping seed")
		return nil
	}

	log.Println("Seeding database with manga data...")

	// Try to load from collected data file first
	if err := loadFromJSONFile(db, "data/manga_collection.json"); err == nil {
		log.Println("✅ Loaded data from manga_collection.json")
		return nil
	}

	// Fallback to basic seed data
	log.Println("Using basic seed data (run 'go run scripts/collect_data.go' for full collection)")
	return seedBasicData(db)
}

// loadFromJSONFile loads manga from JSON file
func loadFromJSONFile(db *sql.DB, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	var manga []struct {
		ID            string   `json:"id"`
		Title         string   `json:"title"`
		Author        string   `json:"author"`
		Genres        []string `json:"genres"`
		Status        string   `json:"status"`
		TotalChapters int      `json:"total_chapters"`
		Description   string   `json:"description"`
		CoverURL      string   `json:"cover_url"`
		MangaURL      string   `json:"manga_url"`
		Year          int      `json:"year"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&manga); err != nil {
		return err
	}

	stmt, err := db.Prepare(`
		INSERT INTO manga (id, title, author, genres, status, total_chapters, description, cover_url, manga_url, year)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range manga {
		genresJSON, _ := json.Marshal(m.Genres)
		_, err := stmt.Exec(
			m.ID,
			m.Title,
			m.Author,
			string(genresJSON),
			m.Status,
			m.TotalChapters,
			m.Description,
			m.CoverURL,
			m.MangaURL,
			m.Year,
		)
		if err != nil {
			log.Printf("Warning: Failed to insert %s: %v", m.Title, err)
		}
	}

	log.Printf("Successfully loaded %d manga from JSON file", len(manga))
	return nil
}

// seedBasicData seeds basic fallback data
func seedBasicData(db *sql.DB) error {

	// Sample manga data
	mangaData := []struct {
		ID            string
		Title         string
		Author        string
		Genres        string
		Status        string
		TotalChapters int
		Description   string
		CoverURL      string
		MangaURL      string
		Year          int
	}{
		{
			ID:            "one-piece",
			Title:         "One Piece",
			Author:        "Oda Eiichiro",
			Genres:        `["Action","Adventure","Comedy","Drama","Shounen"]`,
			Status:        "ongoing",
			TotalChapters: 1100,
			Description:   "Monkey D. Luffy explores the Grand Line in search of the treasure One Piece to become the Pirate King.",
			Year:          1997,
		},
		{
			ID:            "naruto",
			Title:         "Naruto",
			Author:        "Kishimoto Masashi",
			Genres:        `["Action","Adventure","Shounen","Martial Arts"]`,
			Status:        "completed",
			TotalChapters: 700,
			Description:   "Naruto Uzumaki, a young ninja who seeks recognition and dreams of becoming the Hokage.",
			Year:          1999,
		},
		{
			ID:            "attack-on-titan",
			Title:         "Attack on Titan",
			Author:        "Isayama Hajime",
			Genres:        `["Action","Drama","Fantasy","Shounen","Tragedy"]`,
			Status:        "completed",
			TotalChapters: 139,
			Description:   "Humanity fights for survival against giant humanoid Titans.",
			Year:          2009,
		},
		{
			ID:            "death-note",
			Title:         "Death Note",
			Author:        "Ohba Tsugumi",
			Genres:        `["Mystery","Psychological","Supernatural","Thriller"]`,
			Status:        "completed",
			TotalChapters: 108,
			Description:   "A high school student discovers a supernatural notebook that allows him to kill anyone.",
			Year:          2003,
		},
		{
			ID:            "demon-slayer",
			Title:         "Demon Slayer: Kimetsu no Yaiba",
			Author:        "Gotouge Koyoharu",
			Genres:        `["Action","Adventure","Historical","Shounen","Supernatural"]`,
			Status:        "completed",
			TotalChapters: 205,
			Description:   "A boy becomes a demon slayer to avenge his family and cure his sister.",
			Year:          2016,
		},
	}

	// Insert manga data
	stmt, err := db.Prepare(`
		INSERT INTO manga (id, title, author, genres, status, total_chapters, description, cover_url, manga_url, year)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, manga := range mangaData {
		_, err := stmt.Exec(
			manga.ID,
			manga.Title,
			manga.Author,
			manga.Genres,
			manga.Status,
			manga.TotalChapters,
			manga.Description,
			manga.CoverURL,
			manga.MangaURL,
			manga.Year,
		)
		if err != nil {
			return fmt.Errorf("failed to insert manga %s: %w", manga.Title, err)
		}
	}

	log.Printf("Successfully seeded %d manga entries", len(mangaData))
	return nil
}