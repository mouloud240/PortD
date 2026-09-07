package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/portd/internal/httperr"
	portsvc "github.com/portd/internal/ports"
	projectsvc "github.com/portd/internal/projects"
)

func (h *ProjectsHandler) PromotePortPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	slug := r.PathValue("slug")
	port, ok := parsePort(r.FormValue("port"))
	if !ok {
		return h.portBack(w, r, slug, portsvc.ErrOutOfRange)
	}
	if err := h.service.PromotePort(r.Context(), slug, port); err != nil {
		return h.portBack(w, r, slug, err)
	}
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) AllocatePortsPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	slug := r.PathValue("slug")
	n := 1
	if raw := strings.TrimSpace(r.FormValue("count")); raw != "" {
		var err error
		n, err = strconv.Atoi(raw)
		if err != nil {
			return h.portBack(w, r, slug, projectsvc.ErrInvalid)
		}
	}
	if err := h.service.AddPorts(r.Context(), slug, n); err != nil {
		return h.portBack(w, r, slug, err)
	}
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) ReleasePortPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	slug := r.PathValue("slug")
	port, ok := parsePort(r.FormValue("port"))
	if !ok {
		return h.portBack(w, r, slug, portsvc.ErrOutOfRange)
	}
	if err := h.service.ReleasePort(r.Context(), slug, port); err != nil {
		return h.portBack(w, r, slug, err)
	}
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) ClaimPortPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	slug := r.PathValue("slug")
	port, err := strconv.Atoi(strings.TrimSpace(r.FormValue("port")))
	if err != nil {
		return h.portBack(w, r, slug, projectsvc.ErrInvalid)
	}
	if err := h.service.ClaimPort(r.Context(), slug, port); err != nil {
		return h.portBack(w, r, slug, err)
	}
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) AddHealthcheckPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	slug := r.PathValue("slug")
	expected := 0
	if raw := strings.TrimSpace(r.FormValue("expected_status")); raw != "" {
		var err error
		expected, err = strconv.Atoi(raw)
		if err != nil {
			return h.healthBack(w, r, slug, projectsvc.ErrInvalid)
		}
	}
	if _, err := h.service.AddHealthcheck(r.Context(), slug, r.FormValue("endpoint"), expected); err != nil {
		return h.healthBack(w, r, slug, err)
	}
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) RemoveHealthcheckPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Invalid form submission.", err)
	}
	slug := r.PathValue("slug")
	if err := h.service.RemoveHealthcheck(r.Context(), slug, r.PathValue("id")); err != nil {
		return h.healthBack(w, r, slug, err)
	}
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}
