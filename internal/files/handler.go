package files

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/chat"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/go-chi/chi/v5"
)

// Broadcaster delivers events to a set of users over WebSocket. It is satisfied
// by *ws.Hub; declared here as an interface to avoid importing the ws package.
type Broadcaster interface {
	SendJSON(userIDs []int64, eventType string, payload any)
}

type Handler struct {
	service     *Service
	chatService *chat.Service
	broadcaster Broadcaster
}

func NewHandler(service *Service, chatService *chat.Service, broadcaster Broadcaster) *Handler {
	return &Handler{service: service, chatService: chatService, broadcaster: broadcaster}
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	chatID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	if err := h.chatService.EnsureMember(r.Context(), chatID, claims.UserID); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.service.maxSize+1024*1024) // overhead for multipart

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid file upload"})
		return
	}
	defer file.Close()

	result, err := h.service.Upload(r.Context(), chatID, claims.UserID, header.Filename, header.Size, file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	h.broadcastUpload(r.Context(), chatID, claims.UserID, result)

	writeJSON(w, http.StatusCreated, result)
}

// broadcastUpload emits a message.new event to every member of the chat so the
// uploaded file message appears in real time for the sender and other members.
// It is best-effort: failures are logged but never fail the upload response.
func (h *Handler) broadcastUpload(ctx context.Context, chatID, senderID int64, result *UploadResult) {
	if h.broadcaster == nil {
		return
	}
	memberIDs, err := h.chatService.GetMemberIDs(ctx, chatID)
	if err != nil {
		log.Printf("files: broadcast get members: %v", err)
		return
	}
	senderName, err := h.service.GetSenderDisplayName(ctx, senderID)
	if err != nil {
		log.Printf("files: broadcast get sender name: %v", err)
		return
	}

	att := result.Attachment
	var thumbnailPath *string
	if att.ThumbnailPath.Valid {
		thumbnailPath = &att.ThumbnailPath.String
	}

	h.broadcaster.SendJSON(memberIDs, "message.new", map[string]any{
		"id":          result.Message.ID,
		"chat_id":     result.Message.ChatID,
		"sender_id":   result.Message.SenderID,
		"sender_name": senderName,
		"content":     nil,
		"reply_to_id": nil,
		"attachments": []map[string]any{
			{
				"id":             att.ID,
				"message_id":     att.MessageID,
				"file_name":      att.FileName,
				"file_size":      att.FileSize,
				"mime_type":      att.MimeType,
				"storage_path":   att.StoragePath,
				"thumbnail_path": thumbnailPath,
				"created_at":     att.CreatedAt.Time,
			},
		},
		"created_at": result.Message.CreatedAt.Time,
		"client_id":  "",
	})
}

// authorizeAttachment resolves the attachment and confirms the requesting user
// is a member of its owning chat. It returns false for both missing
// attachments and non-members so the handler can respond uniformly without
// leaking whether a given file exists.
func (h *Handler) authorizeAttachment(r *http.Request, fileID int64) (dbq.GetAttachmentAccessContextRow, bool) {
	claims := auth.GetClaims(r.Context())
	att, err := h.service.GetAttachmentAccessContext(r.Context(), fileID)
	if err != nil {
		return dbq.GetAttachmentAccessContextRow{}, false
	}
	if err := h.chatService.EnsureMember(r.Context(), att.ChatID, claims.UserID); err != nil {
		return dbq.GetAttachmentAccessContextRow{}, false
	}
	return att, true
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	fileID, _ := strconv.ParseInt(chi.URLParam(r, "fileID"), 10, 64)

	att, ok := h.authorizeAttachment(r, fileID)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member"})
		return
	}

	url, err := h.service.GetFileURLByPath(r.Context(), att.StoragePath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	// Return the presigned URL as JSON rather than a 302 redirect: the frontend
	// fetches this with its bearer token and then loads the URL directly in an
	// <img>/<a>, which works cross-origin without storage CORS configuration.
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (h *Handler) Thumbnail(w http.ResponseWriter, r *http.Request) {
	fileID, _ := strconv.ParseInt(chi.URLParam(r, "fileID"), 10, 64)

	att, ok := h.authorizeAttachment(r, fileID)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member"})
		return
	}
	if !att.ThumbnailPath.Valid {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no thumbnail"})
		return
	}

	url, err := h.service.GetThumbURLByPath(r.Context(), att.ThumbnailPath.String)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
