package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

// LogArticleEvent is a fire-and-forget helper for recording article history.
func LogArticleEvent(ctx context.Context, q *db.Queries, claims auth.Claims, articleID pgtype.UUID, eventType, description string, meta map[string]string) {
	metadata, _ := json.Marshal(meta)
	q.CreateArticleEvent(ctx, db.CreateArticleEventParams{
		GroupID:     claims.GroupID,
		ArticleID:   articleID,
		ActorID:     claims.MemberID,
		EventType:   eventType,
		Description: description,
		Metadata:    metadata,
	})
}

func formatUUID(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
