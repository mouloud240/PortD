package auth

import (
	"html/template"
	"net/http"
	"time"
)

var loginTemplate = template.Must(template.New("login").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Sign in · PortD</title><style>body{font:16px system-ui;margin:12vh auto;max-width:360px;padding:24px;background:#f5f8fb;color:#172b4d}main{background:#fff;border:1px solid #dbe3ed;border-radius:7px;padding:24px}label{display:grid;gap:6px;margin-top:16px;font-weight:700}input{padding:9px;border:1px solid #b8c5d5;border-radius:4px;font:inherit}button{width:100%;margin-top:22px;padding:10px;border:0;border-radius:7px;background:#003da5;color:#fff;font:inherit;font-weight:700}.error{color:#bd2d2d}</style></head><body><main><h1>PortD</h1><p>Sign in to manage internship projects.</p>{{if .Error}}<p class="error" role="alert">{{.Error}}</p>{{end}}<form method="post" action="/login"><label>Username<input name="username" required autocomplete="username"></label><label>Password<input type="password" name="password" required autocomplete="current-password"></label><button>Sign in</button></form></main></body></html>`))

type loginData struct{ Error string }

func (s *Service) LoginPage(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return loginTemplate.Execute(w, loginData{})
}

func (s *Service) LoginPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}
	session, err := s.Login(r.Context(), r.FormValue("username"), r.FormValue("password"))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return loginTemplate.Execute(w, loginData{Error: "Invalid username or password."})
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
