package message

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/chat"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Broadcaster sends events to users via WebSocket.
type Broadcaster interface {
	SendJSON(userIDs []int64, eventType string, payload any)
}

type Handler struct {
	service     *Service
	chatService *chat.Service
	queries     dbq.Querier
	broadcaster Broadcaster
}

func NewHandler(service *Service, chatService *chat.Service, queries dbq.Querier, broadcaster Broadcaster) *Handler {
	return &Handler{service: service, chatService: chatService, queries: queries, broadcaster: broadcaster}
}

func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	chatID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	if err := h.chatService.EnsureMember(r.Context(), chatID, claims.UserID); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member"})
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := int32(50)
	if limitStr != "" {
		if l, err := strconv.ParseInt(limitStr, 10, 32); err == nil && l > 0 && l <= 100 {
			limit = int32(l)
		}
	}

	beforeStr := r.URL.Query().Get("before")
	if beforeStr != "" {
		beforeID, err := strconv.ParseInt(beforeStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid before parameter"})
			return
		}
		msgs, err := h.queries.ListMessagesByChatBefore(r.Context(), dbq.ListMessagesByChatBeforeParams{
			ChatID: chatID,
			ID:     beforeID,
			Limit:  limit,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		ids := make([]int64, len(msgs))
		for i, m := range msgs {
			ids[i] = m.ID
		}
		attMap, err := h.attachmentsByMessageID(r.Context(), ids)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		dtos := make([]MessageDTO, len(msgs))
		for i, m := range msgs {
			dtos[i] = buildMessageDTO(m.ID, m.ChatID, m.SenderID, m.SenderUsername, m.SenderDisplayName, m.Content, m.ReplyToID, m.CreatedAt, m.UpdatedAt, attMap[m.ID])
		}
		writeJSON(w, http.StatusOK, dtos)
		return
	}

	msgs, err := h.queries.ListMessagesByChatLatest(r.Context(), dbq.ListMessagesByChatLatestParams{
		ChatID: chatID,
		Limit:  limit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	ids := make([]int64, len(msgs))
	for i, m := range msgs {
		ids[i] = m.ID
	}
	attMap, err := h.attachmentsByMessageID(r.Context(), ids)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	dtos := make([]MessageDTO, len(msgs))
	for i, m := range msgs {
		dtos[i] = buildMessageDTO(m.ID, m.ChatID, m.SenderID, m.SenderUsername, m.SenderDisplayName, m.Content, m.ReplyToID, m.CreatedAt, m.UpdatedAt, attMap[m.ID])
	}
	writeJSON(w, http.StatusOK, dtos)
}

func (h *Handler) GetMessage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	chatID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	msgID, _ := strconv.ParseInt(chi.URLParam(r, "mid"), 10, 64)

	if err := h.chatService.EnsureMember(r.Context(), chatID, claims.UserID); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member"})
		return
	}

	msg, err := h.queries.GetMessageByID(r.Context(), msgID)
	// Scope the message to the chat in the URL: membership was checked against
	// chatID, so a message belonging to a different chat must not leak (its
	// content or attachment metadata) even to a member of the URL's chat.
	if err != nil || msg.ChatID != chatID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "message not found"})
		return
	}
	attMap, err := h.attachmentsByMessageID(r.Context(), []int64{msg.ID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	dto := buildMessageDTO(msg.ID, msg.ChatID, msg.SenderID, msg.SenderUsername, msg.SenderDisplayName, msg.Content, msg.ReplyToID, msg.CreatedAt, msg.UpdatedAt, attMap[msg.ID])
	writeJSON(w, http.StatusOK, dto)
}

func (h *Handler) EditMessage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	chatID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	msgID, _ := strconv.ParseInt(chi.URLParam(r, "mid"), 10, 64)

	if err := h.chatService.EnsureMember(r.Context(), chatID, claims.UserID); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member"})
		return
	}

	msg, err := h.queries.GetMessageByID(r.Context(), msgID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "message not found"})
		return
	}
	if msg.SenderID != claims.UserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "can only edit own messages"})
		return
	}
	if time.Since(msg.CreatedAt.Time) > 24*time.Hour {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "can only edit messages within 24 hours"})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	updated, err := h.queries.UpdateMessageContent(r.Context(), dbq.UpdateMessageContentParams{
		ID:      msgID,
		Content: pgtype.Text{String: req.Content, Valid: true},
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	// Broadcast update to chat members.
	if memberIDs, err := h.chatService.GetMemberIDs(r.Context(), chatID); err == nil {
		h.broadcaster.SendJSON(memberIDs, "message.updated", map[string]any{
			"id":         updated.ID,
			"chat_id":    updated.ChatID,
			"content":    req.Content,
			"updated_at": updated.UpdatedAt.Time,
		})
	}

	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	chatID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	msgID, _ := strconv.ParseInt(chi.URLParam(r, "mid"), 10, 64)

	if err := h.chatService.EnsureMember(r.Context(), chatID, claims.UserID); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member"})
		return
	}

	msg, err := h.queries.GetMessageByID(r.Context(), msgID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "message not found"})
		return
	}
	if msg.SenderID != claims.UserID && !claims.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "can only delete own messages"})
		return
	}

	// Soft delete.
	if err := h.queries.SoftDeleteMessage(r.Context(), msgID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	// Delete attachments from S3 + DB.
	h.queries.DeleteAttachmentsByMessage(r.Context(), msgID)

	// Broadcast deletion.
	if memberIDs, err := h.chatService.GetMemberIDs(r.Context(), chatID); err == nil {
		h.broadcaster.SendJSON(memberIDs, "message.deleted", map[string]any{
			"id":      msgID,
			"chat_id": chatID,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
