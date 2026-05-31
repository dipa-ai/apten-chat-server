package files

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/chat"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service     *Service
	chatService *chat.Service
}

func NewHandler(service *Service, chatService *chat.Service) *Handler {
	return &Handler{service: service, chatService: chatService}
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
	writeJSON(w, http.StatusCreated, result)
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
	http.Redirect(w, r, url, http.StatusFound)
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
	http.Redirect(w, r, url, http.StatusFound)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
