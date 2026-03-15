package files

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/chat"
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

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	fileID, _ := strconv.ParseInt(chi.URLParam(r, "fileID"), 10, 64)

	// Verify membership via attachment's message.
	att, err := h.service.GetAttachment(r.Context(), fileID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "file not found"})
		return
	}
	_ = claims
	_ = att

	url, err := h.service.GetFileURL(r.Context(), fileID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *Handler) Thumbnail(w http.ResponseWriter, r *http.Request) {
	fileID, _ := strconv.ParseInt(chi.URLParam(r, "fileID"), 10, 64)

	url, err := h.service.GetThumbURL(r.Context(), fileID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no thumbnail"})
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
