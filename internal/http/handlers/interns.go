package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	activitysvc "github.com/portd/internal/activity"
	"github.com/portd/internal/httperr"
	internsvc "github.com/portd/internal/interns"
	"github.com/portd/views/pages"
)

func (h *InternsHandler) ListPage(w http.ResponseWriter, r *http.Request) error {
	search := r.URL.Query().Get("q")
	state := r.URL.Query().Get("state")
	var active int64 = -1
	switch state {
	case "active":
		active = 1
	case "inactive":
		active = 0
	default:
		state = "all"
	}
	items, err := h.service.List(r.Context(), search, active)
	if err != nil {
		return err
	}
	return httperr.Render(w, r, http.StatusOK, pages.InternsPage("Stagiaires", r.URL.Path, search, state, pages.ListItems(items)))
}

func (h *InternsHandler) NewPage(w http.ResponseWriter, r *http.Request) error {
	return httperr.Render(w, r, http.StatusOK, pages.InternFormPage(pages.InternFormData{
		Title:  "Nouveau stagiaire",
		Path:   "/interns",
		Action: "/interns",
		Active: true,
		IsNew:  true,
	}))
}

func (h *InternsHandler) CreatePost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
	}
	form := pages.InternFormData{
		Title:      "Nouveau stagiaire",
		Path:       "/interns",
		Action:     "/interns",
		FullName:   r.FormValue("full_name"),
		Identifier: r.FormValue("identifier"),
		Email:      r.FormValue("email"),
		Active:     true,
		IsNew:      true,
	}
	intern, err := h.service.Create(r.Context(), form.FullName, form.Email, form.Identifier, r.FormValue("password"))
	if err != nil {
		switch {
		case errors.Is(err, internsvc.ErrInvalid):
			form.Error = "Le nom complet, le nom d'utilisateur et le mot de passe sont requis."
			return httperr.Render(w, r, http.StatusUnprocessableEntity, pages.InternFormPage(form))
		case errors.Is(err, internsvc.ErrConflict):
			form.Error = "Ce nom d'utilisateur ou cet e-mail est déjà pris."
			return httperr.Render(w, r, http.StatusConflict, pages.InternFormPage(form))
		default:
			return err
		}
	}
	http.Redirect(w, r, "/interns/"+intern.ID, http.StatusSeeOther)
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.InternCreate,
		EntityType: "intern",
		EntityID:   intern.ID,
		Detail:     intern.FullName,
	})
	return nil
}

func (h *InternsHandler) DetailPage(w http.ResponseWriter, r *http.Request) error {
	intern, err := h.service.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, internsvc.ErrNotFound) {
			return httperr.NotFound("Stagiaire introuvable.", err)
		}
		return err
	}
	return httperr.Render(w, r, http.StatusOK, pages.InternFormPage(pages.InternFormData{
		Title:      "Modifier le stagiaire",
		Path:       "/interns",
		Action:     "/interns/" + intern.ID,
		ID:         intern.ID,
		FullName:   intern.FullName,
		Identifier: intern.Identifier.String,
		Email:      intern.Email.String,
		Active:     intern.Active == 1,
	}))
}

func (h *InternsHandler) UpdatePost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
	}
	active, _ := strconv.ParseBool(r.FormValue("active"))
	form := pages.InternFormData{
		Title:      "Modifier le stagiaire",
		Path:       "/interns",
		Action:     "/interns/" + r.PathValue("id"),
		ID:         r.PathValue("id"),
		FullName:   r.FormValue("full_name"),
		Identifier: r.FormValue("identifier"),
		Email:      r.FormValue("email"),
		Active:     active,
	}
	intern, err := h.service.Update(r.Context(), form.ID, form.FullName, form.Email, form.Identifier, active)
	if err != nil {
		switch {
		case errors.Is(err, internsvc.ErrInvalid):
			form.Error = "Le nom complet et le nom d'utilisateur sont requis."
			return httperr.Render(w, r, http.StatusUnprocessableEntity, pages.InternFormPage(form))
		case errors.Is(err, internsvc.ErrConflict):
			form.Error = "Ce nom d'utilisateur ou cet e-mail est déjà pris."
			return httperr.Render(w, r, http.StatusConflict, pages.InternFormPage(form))
		case errors.Is(err, sql.ErrNoRows):
			return httperr.NotFound("Stagiaire introuvable.", err)
		default:
			return err
		}
	}
	http.Redirect(w, r, "/interns/"+intern.ID, http.StatusSeeOther)
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.InternUpdate,
		EntityType: "intern",
		EntityID:   intern.ID,
		Detail:     intern.FullName,
	})
	return nil
}
