package manga

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"mangahub/pkg/models"
)

var (
	ErrMangaNotFound = errors.New("manga not found")
	ErrProgressNotFound = errors.New("progress not found")
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Search searches for manga by title or author
func (r *Repository) Search(query, genre, status string, limit, offset int) ([]*models.Manga, error) {
	if limit <= 0 {
		limit = 20
	}

	var conditions []string
	var args []interface{}

	if query != "" {
		conditions = append(conditions, "(title LIKE ? OR author LIKE ?)")
		searchTerm := "%" + query + "%"
		args = append(args, searchTerm, searchTerm)
	}

	if genre != "" {
		conditions = append(conditions, "genres LIKE ?")
		args = append(args, "%"+genre+"%")
	}

	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}

	sqlQuery := "SELECT id, title, author, genres, status, total_chapters, description, cover_url, year FROM manga"
	if len(conditions) > 0 {
		sqlQuery += " WHERE " + strings.Join(conditions, " AND ")
	}
	sqlQuery += " ORDER BY title LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search manga: %w", err)
	}
	defer rows.Close()

	var mangas []*models.Manga
	for rows.Next() {
		manga := &models.Manga{}
		err := rows.Scan(
			&manga.ID,
			&manga.Title,
			&manga.Author,
			&manga.Genres,
			&manga.Status,
			&manga.TotalChapters,
			&manga.Description,
			&manga.CoverURL,
			&manga.Year,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan manga: %w", err)
		}
		mangas = append(mangas, manga)
	}

	return mangas, nil
}

// GetByID retrieves a manga by ID
func (r *Repository) GetByID(id string) (*models.Manga, error) {
	manga := &models.Manga{}
	query := `SELECT id, title, author, genres, status, total_chapters, description, cover_url, year 
			  FROM manga WHERE id = ?`
	err := r.db.QueryRow(query, id).Scan(
		&manga.ID,
		&manga.Title,
		&manga.Author,
		&manga.Genres,
		&manga.Status,
		&manga.TotalChapters,
		&manga.Description,
		&manga.CoverURL,
		&manga.Year,
	)
	if err == sql.ErrNoRows {
		return nil, ErrMangaNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get manga: %w", err)
	}
	return manga, nil
}

// GetUserLibrary retrieves user's manga library
func (r *Repository) GetUserLibrary(userID, status string) ([]*models.UserProgress, error) {
	query := `
		SELECT up.user_id, up.manga_id, up.current_chapter, up.status, up.rating, up.updated_at, up.started_at
		FROM user_progress up
		WHERE up.user_id = ?
	`
	args := []interface{}{userID}

	if status != "" {
		query += " AND up.status = ?"
		args = append(args, status)
	}

	query += " ORDER BY up.updated_at DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get library: %w", err)
	}
	defer rows.Close()

	var library []*models.UserProgress
	for rows.Next() {
		progress := &models.UserProgress{}
		err := rows.Scan(
			&progress.UserID,
			&progress.MangaID,
			&progress.CurrentChapter,
			&progress.Status,
			&progress.Rating,
			&progress.UpdatedAt,
			&progress.StartedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan progress: %w", err)
		}
		library = append(library, progress)
	}

	return library, nil
}

// AddToLibrary adds manga to user's library
func (r *Repository) AddToLibrary(progress *models.UserProgress) error {
	query := `
		INSERT INTO user_progress (user_id, manga_id, current_chapter, status, rating, updated_at, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, manga_id) DO UPDATE SET
			current_chapter = excluded.current_chapter,
			status = excluded.status,
			rating = excluded.rating,
			updated_at = excluded.updated_at
	`
	_, err := r.db.Exec(
		query,
		progress.UserID,
		progress.MangaID,
		progress.CurrentChapter,
		progress.Status,
		progress.Rating,
		progress.UpdatedAt,
		progress.StartedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to add to library: %w", err)
	}
	return nil
}

// UpdateProgress updates reading progress
func (r *Repository) UpdateProgress(userID, mangaID string, chapter int) error {
	query := `
		UPDATE user_progress 
		SET current_chapter = ?, updated_at = ?
		WHERE user_id = ? AND manga_id = ?
	`
	result, err := r.db.Exec(query, chapter, time.Now(), userID, mangaID)
	if err != nil {
		return fmt.Errorf("failed to update progress: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrProgressNotFound
	}

	return nil
}

// GetProgress retrieves user's progress for a manga
func (r *Repository) GetProgress(userID, mangaID string) (*models.UserProgress, error) {
	progress := &models.UserProgress{}
	query := `
		SELECT user_id, manga_id, current_chapter, status, rating, updated_at, started_at
		FROM user_progress
		WHERE user_id = ? AND manga_id = ?
	`
	err := r.db.QueryRow(query, userID, mangaID).Scan(
		&progress.UserID,
		&progress.MangaID,
		&progress.CurrentChapter,
		&progress.Status,
		&progress.Rating,
		&progress.UpdatedAt,
		&progress.StartedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrProgressNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get progress: %w", err)
	}
	return progress, nil
}

// RemoveFromLibrary removes manga from user's library
func (r *Repository) RemoveFromLibrary(userID, mangaID string) error {
	query := `DELETE FROM user_progress WHERE user_id = ? AND manga_id = ?`
	result, err := r.db.Exec(query, userID, mangaID)
	if err != nil {
		return fmt.Errorf("failed to remove from library: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrProgressNotFound
	}

	return nil
}