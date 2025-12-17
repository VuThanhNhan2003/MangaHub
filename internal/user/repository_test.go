package user

import (
	"testing"
	"time"

	"mangahub/pkg/database"
	"mangahub/pkg/models"
)

func setupTestDB(t *testing.T) *Repository {
	db, err := database.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to setup test database: %v", err)
	}
	
	return NewRepository(db)
}

func TestCreateUser(t *testing.T) {
	repo := setupTestDB(t)
	
	user := &models.User{
		ID:           "test-user-1",
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
		CreatedAt:    time.Now(),
	}
	
	err := repo.Create(user)
	if err != nil {
		t.Errorf("Failed to create user: %v", err)
	}
	
	// Try to create duplicate username
	user2 := &models.User{
		ID:           "test-user-2",
		Username:     "testuser",
		Email:        "test2@example.com",
		PasswordHash: "hashed_password",
		CreatedAt:    time.Now(),
	}
	
	err = repo.Create(user2)
	if err != ErrUsernameExists {
		t.Errorf("Expected ErrUsernameExists, got: %v", err)
	}
}

func TestGetUserByUsername(t *testing.T) {
	repo := setupTestDB(t)
	
	// Create test user
	user := &models.User{
		ID:           "test-user-1",
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
		CreatedAt:    time.Now(),
	}
	
	err := repo.Create(user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	
	// Get user by username
	retrieved, err := repo.GetByUsername("testuser")
	if err != nil {
		t.Errorf("Failed to get user: %v", err)
	}
	
	if retrieved.Username != user.Username {
		t.Errorf("Expected username %s, got %s", user.Username, retrieved.Username)
	}
	
	if retrieved.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, retrieved.Email)
	}
	
	// Try to get non-existent user
	_, err = repo.GetByUsername("nonexistent")
	if err != ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got: %v", err)
	}
}

func TestGetUserByEmail(t *testing.T) {
	repo := setupTestDB(t)
	
	// Create test user
	user := &models.User{
		ID:           "test-user-1",
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
		CreatedAt:    time.Now(),
	}
	
	err := repo.Create(user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	
	// Get user by email
	retrieved, err := repo.GetByEmail("test@example.com")
	if err != nil {
		t.Errorf("Failed to get user: %v", err)
	}
	
	if retrieved.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, retrieved.Email)
	}
}

func TestGetUserByID(t *testing.T) {
	repo := setupTestDB(t)
	
	// Create test user
	user := &models.User{
		ID:           "test-user-1",
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
		CreatedAt:    time.Now(),
	}
	
	err := repo.Create(user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	
	// Get user by ID
	retrieved, err := repo.GetByID("test-user-1")
	if err != nil {
		t.Errorf("Failed to get user: %v", err)
	}
	
	if retrieved.ID != user.ID {
		t.Errorf("Expected ID %s, got %s", user.ID, retrieved.ID)
	}
}

func TestCreateUserWithDuplicateEmail(t *testing.T) {
	repo := setupTestDB(t)
	
	user1 := &models.User{
		ID:           "test-user-1",
		Username:     "testuser1",
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
		CreatedAt:    time.Now(),
	}
	
	err := repo.Create(user1)
	if err != nil {
		t.Fatalf("Failed to create first user: %v", err)
	}
	
	// Try to create user with duplicate email
	user2 := &models.User{
		ID:           "test-user-2",
		Username:     "testuser2",
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
		CreatedAt:    time.Now(),
	}
	
	err = repo.Create(user2)
	if err != ErrEmailExists {
		t.Errorf("Expected ErrEmailExists, got: %v", err)
	}
}