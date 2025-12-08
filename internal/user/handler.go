package user

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"mangahub/internal/auth"
	"mangahub/pkg/models"
	"net/http"
)

type Service struct {
	repo      *Repository
	jwtSecret string
}

func NewService(repo *Repository, jwtSecret string) *Service {
	return &Service{
		repo:      repo,
		jwtSecret: jwtSecret,
	}
}

// Register creates a new user account
func (s *Service) Register(req *models.RegisterRequest) (*models.User, error) {
	// Hash password
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		CreatedAt:    time.Now(),
	}

	if err := s.repo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

// Login authenticates a user and returns a JWT token
func (s *Service) Login(req *models.LoginRequest) (string, *models.User, error) {
	// Get user by username
	user, err := s.repo.GetByUsername(req.Username)
	if err != nil {
		return "", nil, fmt.Errorf("invalid credentials")
	}

	// Check password
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		return "", nil, fmt.Errorf("invalid credentials")
	}

	// Generate token
	token, err := auth.GenerateToken(user.ID, user.Username, s.jwtSecret)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return token, user, nil
}

// Handler struct
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register handles user registration
func (h *Handler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "invalid request: " + err.Error(),
		})
		return
	}

	user, err := h.service.Register(&req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err == ErrUsernameExists {
			statusCode = http.StatusConflict
			err = fmt.Errorf("username already exists")
		} else if err == ErrEmailExists {
			statusCode = http.StatusConflict
			err = fmt.Errorf("email already exists")
		}
		c.JSON(statusCode, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, models.Response{
		Success: true,
		Message: "account created successfully",
		Data: gin.H{
			"user_id":  user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// Login handles user login
func (h *Handler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "invalid request: " + err.Error(),
		})
		return
	}

	token, user, err := h.service.Login(&req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "login successful",
		Data: gin.H{
			"token":    token,
			"user_id":  user.ID,
			"username": user.Username,
		},
	})
}

// GetProfile handles getting user profile
func (h *Handler) GetProfile(c *gin.Context) {
	userID := auth.GetUserID(c)
	
	user, err := h.service.repo.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.Response{
			Success: false,
			Error:   "user not found",
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data: gin.H{
			"user_id":    user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"created_at": user.CreatedAt,
		},
	})
}