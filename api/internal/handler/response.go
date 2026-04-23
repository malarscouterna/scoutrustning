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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

func WriteErrorWithParams(w http.ResponseWriter, status int, key string, params map[string]string) {
	WriteJSON(w, status, map[string]any{"error": key, "params": params})
}

// LogArticleEvent is a fire-and-forget helper for recording article history.
func LogArticleEvent(ctx context.Context, q *db.Queries, claims auth.Claims, articleID pgtype.UUID, eventType, description string, meta map[string]string) {
	LogArticleEventWithImages(ctx, q, claims, articleID, eventType, description, meta, nil)
}

// LogArticleEventWithImages records article history with optional attached image IDs.
func LogArticleEventWithImages(ctx context.Context, q *db.Queries, claims auth.Claims, articleID pgtype.UUID, eventType, description string, meta map[string]string, imageIds []string) {
	merged := make(map[string]any, len(meta)+1)
	for k, v := range meta {
		merged[k] = v
	}
	if len(imageIds) > 0 {
		merged["image_ids"] = imageIds
	}
	metadata, _ := json.Marshal(merged)
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
