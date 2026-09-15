package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/portd/internal/auth"
	"github.com/portd/internal/httperr"
)

// RequestLog records one line per HTTP request at info level.
// Static assets are skipped to keep the log readable.
func RequestLog(next httperr.Handler) httperr.Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if strings.HasPrefix(r.URL.Path, "/static/") {
			return next(w, r)
		}
		start := time.Now()
		err := next(w, r)
		status := http.StatusOK
		if rw, ok := w.(interface{ Status() int }); ok {
			status = rw.Status()
		}
		if err != nil {
			if httpErr, ok := err.(*httperr.HTTPError); ok {
				status = httpErr.Status
			} else {
				status = http.StatusInternalServerError
			}
		}
		user := "anonymous"
		if principal, ok := auth.PrincipalFrom(r.Context()); ok {
			switch {
			case principal.IsAdmin():
				user = "admin"
			case principal.InternID != "":
				user = "intern:" + principal.InternID
			}
		}
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
			"user", user,
		)
		return err
	}
}
