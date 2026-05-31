package files

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/image/draw"
)

const (
	maxThumbnailDim = 300
	presignExpiry   = 15 * time.Minute
)

var imageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

type Service struct {
	s3      *S3Client
	queries dbq.Querier
	maxSize int64
}

func NewService(s3 *S3Client, queries dbq.Querier, maxSize int64) *Service {
	return &Service{s3: s3, queries: queries, maxSize: maxSize}
}

type UploadResult struct {
	Message    dbq.Message    `json:"message"`
	Attachment dbq.Attachment `json:"attachment"`
}

func (s *Service) Upload(ctx context.Context, chatID, senderID int64, fileName string, fileSize int64, fileData io.Reader) (*UploadResult, error) {
	if fileSize > s.maxSize {
		return nil, fmt.Errorf("file too large (max %d bytes)", s.maxSize)
	}

	// Read first 512 bytes for MIME detection.
	buf := make([]byte, 512)
	n, err := io.ReadFull(fileData, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	buf = buf[:n]
	mimeType := http.DetectContentType(buf)

	// Reconstruct reader with peeked bytes.
	fullReader := io.MultiReader(bytes.NewReader(buf), fileData)

	// Read entire file into memory for S3 upload + thumbnail.
	data, err := io.ReadAll(fullReader)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	ext := filepath.Ext(fileName)
	id := uuid.New().String()
	key := fmt.Sprintf("chats/%d/%d/%02d/%s%s", chatID, now.Year(), now.Month(), id, ext)

	if err := s.s3.Upload(ctx, key, mimeType, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("s3 upload: %w", err)
	}

	var thumbPath pgtype.Text
	if imageTypes[mimeType] {
		thumbKey := fmt.Sprintf("chats/%d/%d/%02d/%s_thumb%s", chatID, now.Year(), now.Month(), id, ext)
		if thumbData, err := generateThumbnail(data); err == nil {
			if err := s.s3.Upload(ctx, thumbKey, "image/jpeg", bytes.NewReader(thumbData)); err == nil {
				thumbPath = pgtype.Text{String: thumbKey, Valid: true}
			}
		}
	}

	// Create message.
	msg, err := s.queries.CreateMessage(ctx, dbq.CreateMessageParams{
		ChatID:   chatID,
		SenderID: senderID,
	})
	if err != nil {
		return nil, err
	}

	// Create attachment.
	att, err := s.queries.CreateAttachment(ctx, dbq.CreateAttachmentParams{
		MessageID:     msg.ID,
		FileName:      fileName,
		FileSize:      fileSize,
		MimeType:      mimeType,
		StoragePath:   key,
		ThumbnailPath: thumbPath,
	})
	if err != nil {
		return nil, err
	}

	return &UploadResult{Message: msg, Attachment: att}, nil
}

// GetAttachmentAccessContext returns the attachment along with its owning
// chat ID so callers can authorize access by chat membership.
func (s *Service) GetAttachmentAccessContext(ctx context.Context, fileID int64) (dbq.GetAttachmentAccessContextRow, error) {
	return s.queries.GetAttachmentAccessContext(ctx, fileID)
}

// GetFileURLByPath presigns a download URL for an already-resolved storage
// path, avoiding a second attachment lookup after authorization.
func (s *Service) GetFileURLByPath(ctx context.Context, storagePath string) (string, error) {
	return s.s3.PresignedGetURL(ctx, storagePath, presignExpiry)
}

// GetThumbURLByPath presigns a thumbnail URL for an already-resolved path.
func (s *Service) GetThumbURLByPath(ctx context.Context, thumbnailPath string) (string, error) {
	return s.s3.PresignedGetURL(ctx, thumbnailPath, presignExpiry)
}

func generateThumbnail(data []byte) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Calculate new dimensions maintaining aspect ratio.
	if w <= maxThumbnailDim && h <= maxThumbnailDim {
		// Already small enough, encode as JPEG.
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 80}); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	var newW, newH int
	if w > h {
		newW = maxThumbnailDim
		newH = h * maxThumbnailDim / w
	} else {
		newH = maxThumbnailDim
		newW = w * maxThumbnailDim / h
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

