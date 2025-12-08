package user

import (
	"database/sql"
	"errors"
	"fmt"

	"mangahub/pkg/models"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUsernameExists    = errors.New("username already exists")
	ErrEmailExists       = errors.New("email already exists")
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create creates a new user
func (r *Repository) Create(user *models.User) error {
	query := `
		INSERT INTO users (id, username, email, password_hash, created_at)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query, user.ID, user.Username, user.Email, user.PasswordHash, user.CreatedAt)
	if err != nil {
		if isUniqueConstraintError(err, "username") {
			return ErrUsernameExists
		}
		if isUniqueConstraintError(err, "email") {
			return ErrEmailExists
		}
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetByUsername retrieves a user by username
func (r *Repository) GetByUsername(username string) (*models.User, error) {
	var user models.User
	query := `SELECT id, username, email, password_hash, created_at FROM users WHERE username = ?`
	err := r.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// GetByEmail retrieves a user by email
func (r *Repository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	query := `SELECT id, username, email, password_hash, created_at FROM users WHERE email = ?`
	err := r.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// GetByID retrieves a user by ID
func (r *Repository) GetByID(id string) (*models.User, error) {
	var user models.User
	query := `SELECT id, username, email, password_hash, created_at FROM users WHERE id = ?`
	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// Helper function to check for unique constraint errors
func isUniqueConstraintError(err error, field string) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return (errMsg != "" && 
		(errMsg == "UNIQUE constraint failed: users."+field ||
		 errMsg == "constraint failed: UNIQUE constraint failed: users."+field))
}