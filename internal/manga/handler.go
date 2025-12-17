package manga

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"mangahub/internal/auth"
	"mangahub/internal/udp"
	"mangahub/pkg/models"
)

type Handler struct {
	repo              *Repository
	progressBroadcast chan models.ProgressUpdate
	udpServer         *udp.Server
}

func NewHandler(repo *Repository, progressBroadcast chan models.ProgressUpdate, udpServer *udp.Server) *Handler {
	return &Handler{
		repo:              repo,
		progressBroadcast: progressBroadcast,
		udpServer:         udpServer,
	}
}

// SearchManga handles manga search
func (h *Handler) SearchManga(c *gin.Context) {
	var req models.SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "invalid request parameters",
		})
		return
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	if req.Page <= 0 {
		req.Page = 1
	}

	offset := (req.Page - 1) * req.Limit

	mangas, err := h.repo.Search(req.Query, req.Genre, req.Status, req.Limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   "failed to search manga",
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data: gin.H{
			"mangas": mangas,
			"page":   req.Page,
			"limit":  req.Limit,
			"count":  len(mangas),
		},
	})
}

// GetManga handles getting manga details
func (h *Handler) GetManga(c *gin.Context) {
	mangaID := c.Param("id")

	manga, err := h.repo.GetByID(mangaID)
	if err != nil {
		if err == ErrMangaNotFound {
			c.JSON(http.StatusNotFound, models.Response{
				Success: false,
				Error:   "manga not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   "failed to get manga",
		})
		return
	}

	// Get user progress if authenticated
	userID := auth.GetUserID(c)
	var progress *models.UserProgress
	if userID != "" {
		progress, _ = h.repo.GetProgress(userID, mangaID)
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data: gin.H{
			"manga":    manga,
			"progress": progress,
		},
	})
}

// GetLibrary handles getting user's library
func (h *Handler) GetLibrary(c *gin.Context) {
	userID := auth.GetUserID(c)
	status := c.Query("status")

	library, err := h.repo.GetUserLibrary(userID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   "failed to get library",
		})
		return
	}

	// Get manga details for each entry
	var libraryWithDetails []gin.H
	for _, progress := range library {
		manga, err := h.repo.GetByID(progress.MangaID)
		if err != nil {
			continue
		}
		libraryWithDetails = append(libraryWithDetails, gin.H{
			"manga":    manga,
			"progress": progress,
		})
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data: gin.H{
			"library": libraryWithDetails,
			"count":   len(libraryWithDetails),
		},
	})
}

// AddToLibrary handles adding manga to library
func (h *Handler) AddToLibrary(c *gin.Context) {
	userID := auth.GetUserID(c)

	var req models.AddToLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "invalid request: " + err.Error(),
		})
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		"reading": true, "completed": true, "plan-to-read": true,
		"on-hold": true, "dropped": true,
	}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "invalid status. must be: reading, completed, plan-to-read, on-hold, or dropped",
		})
		return
	}

	// Validate rating
	if req.Rating < 0 || req.Rating > 10 {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "rating must be between 0 and 10",
		})
		return
	}

	// Validate manga exists
	manga, err := h.repo.GetByID(req.MangaID)
	if err != nil {
		if err == ErrMangaNotFound {
			c.JSON(http.StatusNotFound, models.Response{
				Success: false,
				Error:   "manga not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   "failed to verify manga",
		})
		return
	}

	now := time.Now()
	progress := &models.UserProgress{
		UserID:         userID,
		MangaID:        req.MangaID,
		CurrentChapter: req.CurrentChapter,
		Status:         req.Status,
		Rating:         req.Rating,
		UpdatedAt:      now,
		StartedAt:      now,
	}

	if err := h.repo.AddToLibrary(progress); err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   "failed to add to library",
		})
		return
	}

	// Send UDP notification to user
	if h.udpServer != nil {
		notification := models.Notification{
			Type:      "library_update",
			MangaID:   req.MangaID,
			Message:   "Added " + manga.Title + " to your library",
			Timestamp: time.Now().Unix(),
		}
		h.udpServer.SendNotificationToUser(userID, notification)
	}

	c.JSON(http.StatusCreated, models.Response{
		Success: true,
		Message: "manga added to library",
		Data:    progress,
	})
}

// UpdateProgress handles updating reading progress
func (h *Handler) UpdateProgress(c *gin.Context) {
	userID := auth.GetUserID(c)

	var req models.UpdateProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "invalid request: " + err.Error(),
		})
		return
	}

	// Validate chapter number
	if req.Chapter < 0 {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "chapter must be a positive number",
		})
		return
	}

	// Validate manga exists and get total chapters
	manga, err := h.repo.GetByID(req.MangaID)
	if err != nil {
		if err == ErrMangaNotFound {
			c.JSON(http.StatusNotFound, models.Response{
				Success: false,
				Error:   "manga not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   "failed to verify manga",
		})
		return
	}

	// Validate chapter number against total chapters
	if req.Chapter > manga.TotalChapters {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "chapter number exceeds total chapters",
			Data: gin.H{
				"requested_chapter": req.Chapter,
				"total_chapters":    manga.TotalChapters,
			},
		})
		return
	}

	// Update progress
	if err := h.repo.UpdateProgress(userID, req.MangaID, req.Chapter); err != nil {
		if err == ErrProgressNotFound {
			c.JSON(http.StatusNotFound, models.Response{
				Success: false,
				Error:   "manga not in library. Add it first",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   "failed to update progress",
		})
		return
	}

	// Broadcast progress update via TCP (non-blocking)
	if h.progressBroadcast != nil {
		update := models.ProgressUpdate{
			UserID:    userID,
			MangaID:   req.MangaID,
			Chapter:   req.Chapter,
			Timestamp: time.Now().Unix(),
		}
		select {
		case h.progressBroadcast <- update:
		default:
		}
	}

	// Send UDP notification to user
	if h.udpServer != nil {
		notification := models.Notification{
			Type:      "progress_update",
			MangaID:   req.MangaID,
			Message:   "Updated progress to chapter " + string(rune(req.Chapter)),
			Timestamp: time.Now().Unix(),
		}
		h.udpServer.SendNotificationToUser(userID, notification)
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "progress updated successfully",
		Data: gin.H{
			"manga_id": req.MangaID,
			"chapter":  req.Chapter,
			"manga_title": manga.Title,
		},
	})
}

// RemoveFromLibrary handles removing manga from library
func (h *Handler) RemoveFromLibrary(c *gin.Context) {
	userID := auth.GetUserID(c)
	mangaID := c.Param("id")

	// Get manga info before removing
	manga, _ := h.repo.GetByID(mangaID)

	if err := h.repo.RemoveFromLibrary(userID, mangaID); err != nil {
		if err == ErrProgressNotFound {
			c.JSON(http.StatusNotFound, models.Response{
				Success: false,
				Error:   "manga not in library",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   "failed to remove from library",
		})
		return
	}

	// Send UDP notification
	if h.udpServer != nil && manga != nil {
		notification := models.Notification{
			Type:      "library_update",
			MangaID:   mangaID,
			Message:   "Removed " + manga.Title + " from your library",
			Timestamp: time.Now().Unix(),
		}
		h.udpServer.SendNotificationToUser(userID, notification)
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "manga removed from library",
	})
}