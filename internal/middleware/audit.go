package middleware

import (
	"net"
	"net/http"

	"github.com/Ryohskay/portacconti/internal/repository"
	"github.com/google/uuid"
)

// Audit logs access to sensitive routes.
func Audit(auditRepo repository.AuditRepository, action, targetType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
			// Log after response
			actorIDStr := UserIDFromContext(r.Context())
			var actorID *uuid.UUID
			if id, err := uuid.Parse(actorIDStr); err == nil {
				actorID = &id
			}
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			_ = auditRepo.Log(r.Context(), actorID, action, targetType, nil, ip)
		})
	}
}
