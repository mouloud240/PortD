package auth

import (
	"net/http"
	"time"

	"github.com/portd/internal/httperr"
	"github.com/portd/views/pages"
)

func (s *Service) LoginPage(w http.ResponseWriter, r *http.Request) error {
	return httperr.Render(w, r, http.StatusOK, pages.LoginPage(""))
}

func (s *Service) LoginPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	session, err := s.Login(r.Context(), r.FormValue("username"), r.FormValue("password"))
	if err != nil {
		return httperr.Render(w, r, http.StatusUnauthorized, pages.LoginPage("Invalid username or password."))
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: session.Token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil, Expires: time.Now().Add(s.sessionTTL)})
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	return nil
}

func (s *Service) LogoutPost(w http.ResponseWriter, r *http.Request) error {
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		if err := s.Logout(r.Context(), cookie.Value); err != nil {
			return err
		}
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
	return nil
}
