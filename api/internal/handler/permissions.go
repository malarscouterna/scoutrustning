package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

// Permissions holds the configurable permission levels for a group.
type Permissions struct {
	ImageUpload  string
	Booking      string
	ArticleEdit  string
	IssueResolve string
	ManagerNotes string
}

// Default permissions (match DB defaults).
var defaultPermissions = Permissions{
	ImageUpload:  auth.AccessBook,
	Booking:      auth.AccessBook,
	ArticleEdit:  auth.AccessManager,
	IssueResolve: auth.AccessManager,
	ManagerNotes: auth.AccessManager,
}

// Minimum allowed levels per permission.
var minPermissionLevel = map[string]string{
	"image_upload_role":  auth.AccessView,
	"booking_role":       auth.AccessBook,
	"article_edit_role":  auth.AccessBook,
	"issue_resolve_role": auth.AccessBook,
	"manager_notes_role": auth.AccessTrusted,
}

// ValidatePermissionLevel checks if a level is valid for a given permission key.
func ValidatePermissionLevel(key, level string) bool {
	if !validAccessLevel(level) {
		return false
	}
	min, ok := minPermissionLevel[key]
	if !ok {
		return true
	}
	return auth.AccessAtLeast(level, min)
}

// PermissionCache caches group permissions briefly to avoid DB lookups on every request.
type PermissionCache struct {
	q     *db.Queries
	mu    sync.RWMutex
	cache map[string]cachedPerms
}

type cachedPerms struct {
	perms   Permissions
	expires time.Time
}

func NewPermissionCache(q *db.Queries) *PermissionCache {
	return &PermissionCache{q: q, cache: make(map[string]cachedPerms)}
}

func (pc *PermissionCache) Get(r *http.Request, groupID string) Permissions {
	pc.mu.RLock()
	if c, ok := pc.cache[groupID]; ok && time.Now().Before(c.expires) {
		pc.mu.RUnlock()
		return c.perms
	}
	pc.mu.RUnlock()

	settings, err := pc.q.GetGroupSettings(r.Context(), groupID)
	if err != nil {
		return defaultPermissions
	}

	perms := Permissions{
		ImageUpload:  settings.ImageUploadRole,
		Booking:      settings.BookingRole,
		ArticleEdit:  settings.ArticleEditRole,
		IssueResolve: settings.IssueResolveRole,
		ManagerNotes: settings.ManagerNotesRole,
	}

	pc.mu.Lock()
	pc.cache[groupID] = cachedPerms{perms: perms, expires: time.Now().Add(30 * time.Second)}
	pc.mu.Unlock()

	return perms
}

// Invalidate clears the cache for a group (call after settings update).
func (pc *PermissionCache) Invalidate(groupID string) {
	pc.mu.Lock()
	delete(pc.cache, groupID)
	pc.mu.Unlock()
}

// CheckPermission is a helper for handlers to check a configurable permission.
func CheckPermission(w http.ResponseWriter, claims auth.Claims, required string) bool {
	if !auth.AccessAtLeast(claims.MaxAccess, required) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return false
	}
	return true
}

// RequirePermission returns middleware that checks a configurable permission.
// The permFn extracts the required level from the cached permissions.
func RequirePermission(pc *PermissionCache, permFn func(Permissions) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			perms := pc.Get(r, claims.GroupID)
			required := permFn(perms)
			if !auth.AccessAtLeast(claims.MaxAccess, required) {
				WriteError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
