package handlers

import (
	"net/http"
	"time"

	"github.com/portd/internal/auth"
	"github.com/portd/internal/httperr"
	"github.com/portd/views/pages"
)

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) error {
	return httperr.Render(w, r, http.StatusOK, pages.LoginPage(""))
}

func (h *AuthHandler) LoginPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	session, err := h.service.Login(r.Context(), r.FormValue("username"), r.FormValue("password"))
	if err != nil {
		return httperr.Render(w, r, http.StatusUnauthorized, pages.LoginPage("Invalid username or password."))
	}
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookieName, Value: session.Token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil, Expires: time.Now().Add(h.service.SessionTTL())})
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	return nil
}

func (h *AuthHandler) LogoutPost(w http.ResponseWriter, r *http.Request) error {
	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
		if err := h.service.Logout(r.Context(), cookie.Value); err != nil {
			return err
		}
	}
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookieName, Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
	return nil
}
