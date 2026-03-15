package push

import (
	"encoding/json"
	"net/http"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct {
	service *Service
	queries *dbq.Queries
}

func NewHandler(service *Service, queries *dbq.Queries) *Handler {
	return &Handler{service: service, queries: queries}
}

type subscribeRequest struct {
	Endpoint  string `json:"endpoint"`
	P256dhKey string `json:"p256dh_key"`
	AuthKey   string `json:"auth_key"`
}

func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Endpoint == "" || req.P256dhKey == "" || req.AuthKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "all fields required"})
		return
	}

	userAgent := r.Header.Get("User-Agent")
	sub, err := h.queries.CreatePushSubscription(r.Context(), dbq.CreatePushSubscriptionParams{
		UserID:    claims.UserID,
		Endpoint:  req.Endpoint,
		P256dhKey: req.P256dhKey,
		AuthKey:   req.AuthKey,
		UserAgent: pgtype.Text{String: userAgent, Valid: userAgent != ""},
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusCreated, sub)
}

func (h *Handler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.queries.DeletePushSubscription(r.Context(), dbq.DeletePushSubscriptionParams{
		UserID:   claims.UserID,
		Endpoint: req.Endpoint,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) VAPIDKey(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"vapid_key": h.service.VAPIDPublicKey()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
