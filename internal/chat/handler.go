package chat

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct {
	service *Service
	queries dbq.Querier
}

func NewHandler(service *Service, queries dbq.Querier) *Handler {
	return &Handler{service: service, queries: queries}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Type != "direct" && req.Type != "group" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "type must be 'direct' or 'group'"})
		return
	}

	chat, err := h.service.Create(r.Context(), claims.UserID, req)
	if err != nil {
		if errors.Is(err, ErrDirectChatExists) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}
	writeJSON(w, http.StatusCreated, chat)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	chats, err := h.queries.ListChatsByUser(r.Context(), claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, chats)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	chatID := parseChatID(r)
	if chatID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chat id"})
		return
	}

	if err := h.service.EnsureMember(r.Context(), chatID, claims.UserID); err != nil {
		if errors.Is(err, ErrNotMember) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member"})
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}

	chat, err := h.queries.GetChatByID(r.Context(), chatID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "chat not found"})
		return
	}

	members, err := h.queries.ListChatMembers(r.Context(), chatID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"chat":    chat,
		"members": members,
	})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	chatID := parseChatID(r)
	if chatID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chat id"})
		return
	}

	if err := h.service.EnsureMember(r.Context(), chatID, claims.UserID); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member"})
		return
	}

	var req struct {
		Name      *string `json:"name"`
		AvatarURL *string `json:"avatar_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	params := dbq.UpdateChatParams{ID: chatID}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.AvatarURL != nil {
		params.AvatarUrl = pgtype.Text{String: *req.AvatarURL, Valid: true}
	}

	chat, err := h.queries.UpdateChat(r.Context(), params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	chatID := parseChatID(r)

	if err := h.service.EnsureMember(r.Context(), chatID, claims.UserID); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member"})
		return
	}

	var req struct {
		UserID int64 `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.queries.AddChatMember(r.Context(), dbq.AddChatMemberParams{
		ChatID: chatID,
		UserID: req.UserID,
		Role:   pgtype.Text{String: "member", Valid: true},
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	chatID := parseChatID(r)
	uidStr := chi.URLParam(r, "uid")
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	if err := h.service.EnsureMember(r.Context(), chatID, claims.UserID); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member"})
		return
	}

	if err := h.queries.RemoveChatMember(r.Context(), dbq.RemoveChatMemberParams{
		ChatID: chatID,
		UserID: uid,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseChatID(r *http.Request) int64 {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	return id
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
