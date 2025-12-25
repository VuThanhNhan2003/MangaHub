package grpc

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"mangahub/internal/manga"
	"mangahub/pkg/models"
	pb "mangahub/proto/proto"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedMangaServiceServer
	repo              *manga.Repository
	progressBroadcast chan models.ProgressUpdate
}

func NewServer(repo *manga.Repository, progressBroadcast chan models.ProgressUpdate) *Server {
	return &Server{
		repo:              repo,
		progressBroadcast: progressBroadcast,
	}
}

// GetManga retrieves manga by ID
func (s *Server) GetManga(ctx context.Context, req *pb.GetMangaRequest) (*pb.MangaResponse, error) {
	log.Printf("gRPC GetManga called for ID: %s", req.MangaId)

	m, err := s.repo.GetByID(req.MangaId)
	if err != nil {
		if err == manga.ErrMangaNotFound {
			return nil, status.Error(codes.NotFound, "manga not found")
		}
		return nil, status.Error(codes.Internal, "failed to get manga")
	}

	// Parse genres from JSON string
	var genres []string
	if err := json.Unmarshal([]byte(m.Genres), &genres); err != nil {
		genres = []string{}
	}

	return &pb.MangaResponse{
		Id:            m.ID,
		Title:         m.Title,
		Author:        m.Author,
		Genres:        genres,
		Status:        m.Status,
		TotalChapters: int32(m.TotalChapters),
		Description:   m.Description,
		CoverUrl:      m.CoverURL,
		Year:          int32(m.Year),
	}, nil
}

// SearchManga searches for manga
func (s *Server) SearchManga(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	log.Printf("gRPC SearchManga called with query: %s", req.Query)

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}

	mangas, err := s.repo.Search(req.Query, req.Genre, req.Status, limit, int(req.Offset))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to search manga")
	}

	var results []*pb.MangaResponse
	for _, m := range mangas {
		var genres []string
		json.Unmarshal([]byte(m.Genres), &genres)

		results = append(results, &pb.MangaResponse{
			Id:            m.ID,
			Title:         m.Title,
			Author:        m.Author,
			Genres:        genres,
			Status:        m.Status,
			TotalChapters: int32(m.TotalChapters),
			Description:   m.Description,
			CoverUrl:      m.CoverURL,
			Year:          int32(m.Year),
		})
	}

	return &pb.SearchResponse{
		Mangas:     results,
		TotalCount: int32(len(results)),
	}, nil
}

// UpdateProgress updates reading progress
func (s *Server) UpdateProgress(ctx context.Context, req *pb.UpdateProgressRequest) (*pb.UpdateProgressResponse, error) {
	log.Printf("gRPC UpdateProgress called for user %s, manga %s, chapter %d",
		req.UserId, req.MangaId, req.Chapter)

	// Validate manga exists
	m, err := s.repo.GetByID(req.MangaId)
	if err != nil {
		if err == manga.ErrMangaNotFound {
			return nil, status.Error(codes.NotFound, "manga not found")
		}
		return nil, status.Error(codes.Internal, "failed to verify manga")
	}

	// Validate chapter number
	if int(req.Chapter) > m.TotalChapters {
		return &pb.UpdateProgressResponse{
			Success: false,
			Message: "chapter number exceeds total chapters",
		}, nil
	}

	// Update progress
	err = s.repo.UpdateProgress(req.UserId, req.MangaId, int(req.Chapter))
	if err != nil {
		if err == manga.ErrProgressNotFound {
			return &pb.UpdateProgressResponse{
				Success: false,
				Message: "manga not in library",
			}, nil
		}
		return nil, status.Error(codes.Internal, "failed to update progress")
	}

	// Broadcast progress update via TCP (non-blocking)
	if s.progressBroadcast != nil {
		update := models.ProgressUpdate{
			UserID:    req.UserId,
			MangaID:   req.MangaId,
			Chapter:   int(req.Chapter),
			Timestamp: time.Now().Unix(),
		}
		select {
		case s.progressBroadcast <- update:
		default:
		}
	}

	return &pb.UpdateProgressResponse{
		Success:        true,
		Message:        "progress updated successfully",
		CurrentChapter: req.Chapter,
		UpdatedAt:      time.Now().Unix(),
	}, nil
}

// StartGRPCServer starts the gRPC server
func StartGRPCServer(port string, repo *manga.Repository, progressBroadcast chan models.ProgressUpdate) error {
	// Tạo TCP listener
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}

	// Tạo gRPC server
	grpcServer := grpc.NewServer()

	// Khởi tạo server và đăng ký service
	srv := NewServer(repo, progressBroadcast)
	pb.RegisterMangaServiceServer(grpcServer, srv)

	log.Printf("gRPC server listening on %s", port)

	// Chạy server
	return grpcServer.Serve(lis)
}
