package middleware

import (
	"errors"
	"net/http"

	"github.com/portd/internal/auth"
	"github.com/portd/internal/httperr"
	projectsvc "github.com/portd/internal/projects"
)

func RequireSession(service *auth.Service) func(httperr.Handler) httperr.Handler {
	return func(next httperr.Handler) httperr.Handler {
		return func(w http.ResponseWriter, r *http.Request) error {
			cookie, err := r.Cookie(auth.SessionCookieName)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return nil
			}
			principal, err := service.Session(r.Context(), cookie.Value)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return nil
			}
			return next(w, r.WithContext(auth.WithPrincipal(r.Context(), principal)))
		}
	}
}

func RequireAdmin(service *auth.Service) func(httperr.Handler) httperr.Handler {
	return func(next httperr.Handler) httperr.Handler {
		return RequireSession(service)(func(w http.ResponseWriter, r *http.Request) error {
			principal, _ := auth.PrincipalFrom(r.Context())
			if !principal.IsAdmin() {
				return httperr.Forbidden("Administrator access required", nil)
			}
			return next(w, r)
		})
	}
}

func RequireProjectMember(authService *auth.Service, projectService *projectsvc.Service) func(httperr.Handler) httperr.Handler {
	return func(next httperr.Handler) httperr.Handler {
		return RequireSession(authService)(func(w http.ResponseWriter, r *http.Request) error {
			principal, _ := auth.PrincipalFrom(r.Context())
			ok, err := projectService.CanManage(r.Context(), r.PathValue("slug"), principal)
			if err != nil {
				if errors.Is(err, projectsvc.ErrNotFound) {
					return httperr.NotFound("Project not found.", err)
				}
				return err
			}
			if !ok {
				return httperr.Forbidden("You are not assigned to this project.", nil)
			}
			return next(w, r)
		})
	}
}
