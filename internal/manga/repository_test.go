package manga

import (
	"encoding/json"
	"testing"
	"time"

	"mangahub/pkg/database"
	"mangahub/pkg/models"
)

func setupTestRepo(t *testing.T) *Repository {
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to setup test database: %v", err)
	}

	// Seed with test data
	genres, _ := json.Marshal([]string{"Action", "Adventure", "Shounen"})
	_, err = db.Exec(`
		INSERT INTO manga (id, title, author, genres, status, total_chapters, description, cover_url, manga_url, year)
		VALUES 
			('test-manga-1', 'Test Manga 1', 'Test Author 1', ?, 'ongoing', 100, 'Test description 1', 'https://example.com/cover1.jpg', 'https://example.com/manga1', 2020),
			('test-manga-2', 'Test Manga 2', 'Test Author 2', ?, 'completed', 50, 'Test description 2', 'https://example.com/cover2.jpg', 'https://example.com/manga2', 2019)
	`, string(genres), string(genres))

	if err != nil {
		t.Fatalf("Failed to seed test data: %v", err)
	}

	return NewRepository(db)
}

func TestSearchManga(t *testing.T) {
	repo := setupTestRepo(t)

	tests := []struct {
		name    string
		query   string
		genre   string
		status  string
		limit   int
		offset  int
		wantLen int
	}{
		{
			name:    "Search by title",
			query:   "Test",
			limit:   10,
			offset:  0,
			wantLen: 2,
		},
		{
			name:    "Search by author",
			query:   "Author 1",
			limit:   10,
			offset:  0,
			wantLen: 1,
		},
		{
			name:    "Search with limit",
			query:   "Test",
			limit:   1,
			offset:  0,
			wantLen: 1,
		},
		{
			name:    "Search by status",
			status:  "completed",
			limit:   10,
			offset:  0,
			wantLen: 1,
		},
		{
			name:    "Search by genre",
			genre:   "Action",
			limit:   10,
			offset:  0,
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := repo.Search(tt.query, tt.genre, tt.status, tt.limit, tt.offset)
			if err != nil {
				t.Errorf("Search failed: %v", err)
			}

			if len(results) != tt.wantLen {
				t.Errorf("Expected %d results, got %d", tt.wantLen, len(results))
			}
		})
	}
}

func TestGetByID(t *testing.T) {
	repo := setupTestRepo(t)

	// Test existing manga
	manga, err := repo.GetByID("test-manga-1")
	if err != nil {
		t.Errorf("Failed to get manga: %v", err)
	}

	if manga.Title != "Test Manga 1" {
		t.Errorf("Expected title 'Test Manga 1', got '%s'", manga.Title)
	}

	// Test non-existent manga
	_, err = repo.GetByID("non-existent")
	if err != ErrMangaNotFound {
		t.Errorf("Expected ErrMangaNotFound, got: %v", err)
	}
}

func TestAddToLibrary(t *testing.T) {
	repo := setupTestRepo(t)

	progress := &models.UserProgress{
		UserID:         "test-user-1",
		MangaID:        "test-manga-1",
		CurrentChapter: 10,
		Status:         "reading",
		Rating:         8,
		UpdatedAt:      time.Now(),
		StartedAt:      time.Now(),
	}

	err := repo.AddToLibrary(progress)
	if err != nil {
		t.Errorf("Failed to add to library: %v", err)
	}

	// Verify it was added
	retrieved, err := repo.GetProgress("test-user-1", "test-manga-1")
	if err != nil {
		t.Errorf("Failed to get progress: %v", err)
	}

	if retrieved.CurrentChapter != 10 {
		t.Errorf("Expected chapter 10, got %d", retrieved.CurrentChapter)
	}
}

func TestUpdateProgress(t *testing.T) {
	repo := setupTestRepo(t)

	// First add to library
	progress := &models.UserProgress{
		UserID:         "test-user-1",
		MangaID:        "test-manga-1",
		CurrentChapter: 10,
		Status:         "reading",
		UpdatedAt:      time.Now(),
		StartedAt:      time.Now(),
	}

	err := repo.AddToLibrary(progress)
	if err != nil {
		t.Fatalf("Failed to add to library: %v", err)
	}

	// Update progress
	err = repo.UpdateProgress("test-user-1", "test-manga-1", 20)
	if err != nil {
		t.Errorf("Failed to update progress: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetProgress("test-user-1", "test-manga-1")
	if err != nil {
		t.Errorf("Failed to get progress: %v", err)
	}

	if retrieved.CurrentChapter != 20 {
		t.Errorf("Expected chapter 20, got %d", retrieved.CurrentChapter)
	}

	// Test update for non-existent progress
	err = repo.UpdateProgress("test-user-1", "non-existent", 10)
	if err != ErrProgressNotFound {
		t.Errorf("Expected ErrProgressNotFound, got: %v", err)
	}
}

func TestGetUserLibrary(t *testing.T) {
	repo := setupTestRepo(t)

	// Add multiple manga to library
	progress1 := &models.UserProgress{
		UserID:    "test-user-1",
		MangaID:   "test-manga-1",
		Status:    "reading",
		UpdatedAt: time.Now(),
		StartedAt: time.Now(),
	}

	progress2 := &models.UserProgress{
		UserID:    "test-user-1",
		MangaID:   "test-manga-2",
		Status:    "completed",
		UpdatedAt: time.Now(),
		StartedAt: time.Now(),
	}

	repo.AddToLibrary(progress1)
	repo.AddToLibrary(progress2)

	// Get entire library
	library, err := repo.GetUserLibrary("test-user-1", "")
	if err != nil {
		t.Errorf("Failed to get library: %v", err)
	}

	if len(library) != 2 {
		t.Errorf("Expected 2 items in library, got %d", len(library))
	}

	// Get filtered by status
	reading, err := repo.GetUserLibrary("test-user-1", "reading")
	if err != nil {
		t.Errorf("Failed to get reading manga: %v", err)
	}

	if len(reading) != 1 {
		t.Errorf("Expected 1 reading manga, got %d", len(reading))
	}
}

func TestRemoveFromLibrary(t *testing.T) {
	repo := setupTestRepo(t)

	// Add to library
	progress := &models.UserProgress{
		UserID:    "test-user-1",
		MangaID:   "test-manga-1",
		Status:    "reading",
		UpdatedAt: time.Now(),
		StartedAt: time.Now(),
	}

	repo.AddToLibrary(progress)

	// Remove from library
	err := repo.RemoveFromLibrary("test-user-1", "test-manga-1")
	if err != nil {
		t.Errorf("Failed to remove from library: %v", err)
	}

	// Verify removal
	_, err = repo.GetProgress("test-user-1", "test-manga-1")
	if err != ErrProgressNotFound {
		t.Errorf("Expected ErrProgressNotFound after removal, got: %v", err)
	}

	// Test removing non-existent entry
	err = repo.RemoveFromLibrary("test-user-1", "non-existent")
	if err != ErrProgressNotFound {
		t.Errorf("Expected ErrProgressNotFound, got: %v", err)
	}
}

func TestGetProgress(t *testing.T) {
	repo := setupTestRepo(t)

	// Add to library
	progress := &models.UserProgress{
		UserID:         "test-user-1",
		MangaID:        "test-manga-1",
		CurrentChapter: 15,
		Status:         "reading",
		Rating:         9,
		UpdatedAt:      time.Now(),
		StartedAt:      time.Now(),
	}

	repo.AddToLibrary(progress)

	// Get progress
	retrieved, err := repo.GetProgress("test-user-1", "test-manga-1")
	if err != nil {
		t.Errorf("Failed to get progress: %v", err)
	}

	if retrieved.CurrentChapter != 15 {
		t.Errorf("Expected chapter 15, got %d", retrieved.CurrentChapter)
	}

	if retrieved.Rating != 9 {
		t.Errorf("Expected rating 9, got %d", retrieved.Rating)
	}

	// Test getting non-existent progress
	_, err = repo.GetProgress("test-user-1", "non-existent")
	if err != ErrProgressNotFound {
		t.Errorf("Expected ErrProgressNotFound, got: %v", err)
	}
}

func TestSearchPagination(t *testing.T) {
	repo := setupTestRepo(t)

	// Test pagination
	page1, err := repo.Search("Test", "", "", 1, 0)
	if err != nil {
		t.Errorf("Failed to get page 1: %v", err)
	}

	if len(page1) != 1 {
		t.Errorf("Expected 1 result on page 1, got %d", len(page1))
	}

	page2, err := repo.Search("Test", "", "", 1, 1)
	if err != nil {
		t.Errorf("Failed to get page 2: %v", err)
	}

	if len(page2) != 1 {
		t.Errorf("Expected 1 result on page 2, got %d", len(page2))
	}

	// Verify different results
	if page1[0].ID == page2[0].ID {
		t.Errorf("Pages should have different results")
	}
}
