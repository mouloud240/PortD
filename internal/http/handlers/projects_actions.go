package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	activitysvc "github.com/portd/internal/activity"
	"github.com/portd/internal/httperr"
	portsvc "github.com/portd/internal/ports"
	projectsvc "github.com/portd/internal/projects"
	"github.com/portd/internal/runtime"
)

func (h *ProjectsHandler) DetectPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
	}
	added, err := h.service.Import(r.Context(), r.Form["slugs"])
	if err != nil {
		return err
	}
	for _, slug := range added {
		h.activity.Record(r.Context(), activitysvc.Event{
			EventType:  activitysvc.ProjectCreate,
			EntityType: "project",
			EntityID:   slug,
			Detail:     "detected on disk",
		})
	}
	http.Redirect(w, r, "/projects", http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) PromotePortPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
	}
	slug := r.PathValue("slug")
	port, ok := parsePort(r.FormValue("port"))
	if !ok {
		return h.portBack(w, r, slug, portsvc.ErrOutOfRange)
	}
	if err := h.service.PromotePort(r.Context(), slug, port); err != nil {
		return h.portBack(w, r, slug, err)
	}
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.PortPromote,
		EntityType: "project",
		EntityID:   slug,
		Detail:     "port " + strconv.FormatInt(port, 10),
	})
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) RuntimeStartPost(w http.ResponseWriter, r *http.Request) error {
	slug := r.PathValue("slug")
	project, err := h.service.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, projectsvc.ErrNotFound) {
			return httperr.NotFound("Projet introuvable.", err)
		}
		return err
	}
	status, err := h.runtime.Start(r.Context(), runtime.ManagedProject{
		ID:         project.Project.ID,
		Directory:  project.Project.Directory,
		Executable: project.Project.StartupCommand,
	})
	if err != nil {
		slog.Error("runtime start request failed", "slug", slug, "dir", project.Project.Directory, "file", project.Project.StartupCommand, "error", err)
		h.activity.Record(r.Context(), activitysvc.Event{
			EventType:  activitysvc.RuntimeStart,
			EntityType: "project",
			EntityID:   slug,
			Outcome:    activitysvc.OutcomeFailure,
			Detail:     err.Error(),
		})
		if errors.Is(err, runtime.ErrStartupMissing) {
			return h.runtimeBack(w, r, slug, "Le fichier de démarrage est manquant. Ajoutez le fichier configuré dans le dossier du projet avant de le démarrer.")
		}
		if errors.Is(err, runtime.ErrAlreadyRunning) {
			return h.runtimeBack(w, r, slug, "Ce projet est déjà en cours.")
		}
		return h.runtimeBack(w, r, slug, "Le projet n'a pas pu être démarré.")
	}
	slog.Info("runtime start requested", "slug", slug, "pid", status.PID, "file", status.File)
	if err := h.service.SetRuntimeIntent(r.Context(), slug, true, "running"); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
		defer cancel()
		_ = h.runtime.Stop(cleanupCtx, project.Project.ID)
		return err
	}
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.RuntimeStart,
		EntityType: "project",
		EntityID:   slug,
		Detail:     project.Project.StartupCommand,
	})
	http.Redirect(w, r, "/projects/"+slug, http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) RuntimeStopPost(w http.ResponseWriter, r *http.Request) error {
	slug := r.PathValue("slug")
	project, err := h.service.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, projectsvc.ErrNotFound) {
			return httperr.NotFound("Projet introuvable.", err)
		}
		return err
	}
	before := h.runtime.Status(project.Project.ID)
	if err := h.runtime.Stop(r.Context(), project.Project.ID); err != nil && !errors.Is(err, runtime.ErrNotRunning) {
		slog.Error("runtime stop request failed", "slug", slug, "pid", before.PID, "error", err)
		h.activity.Record(r.Context(), activitysvc.Event{
			EventType:  activitysvc.RuntimeStop,
			EntityType: "project",
			EntityID:   slug,
			Outcome:    activitysvc.OutcomeFailure,
			Detail:     err.Error(),
		})
		return h.runtimeBack(w, r, slug, "Le projet n'a pas pu être arrêté.")
	}
	slog.Info("runtime stop requested", "slug", slug, "pid", before.PID)
	if err := h.service.SetRuntimeIntent(r.Context(), slug, false, "stopped"); err != nil {
		return err
	}
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.RuntimeStop,
		EntityType: "project",
		EntityID:   slug,
	})
	http.Redirect(w, r, "/projects/"+slug, http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) RuntimeConfigurePost(w http.ResponseWriter, r *http.Request) error {
	slug := r.PathValue("slug")
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
	}
	if err := h.service.SetStartupCommand(r.Context(), slug, r.FormValue("startup_command")); err != nil {
		if errors.Is(err, projectsvc.ErrNotFound) {
			return httperr.NotFound("Projet introuvable.", err)
		}
		if errors.Is(err, projectsvc.ErrInvalid) {
			return h.runtimeBack(w, r, slug, "Choisissez un fichier de démarrage avant d'enregistrer.")
		}
		return err
	}
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.RuntimeConfigure,
		EntityType: "project",
		EntityID:   slug,
		Detail:     r.FormValue("startup_command"),
	})
	http.Redirect(w, r, "/projects/"+url.PathEscape(slug), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) runtimeBack(w http.ResponseWriter, r *http.Request, slug, message string) error {
	http.Redirect(w, r, "/projects/"+url.PathEscape(slug)+"?runtime_error="+url.QueryEscape(message), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) AllocatePortsPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
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
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.PortAllocate,
		EntityType: "project",
		EntityID:   slug,
		Detail:     strconv.Itoa(n) + " ports",
	})
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) ReleasePortPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
	}
	slug := r.PathValue("slug")
	port, ok := parsePort(r.FormValue("port"))
	if !ok {
		return h.portBack(w, r, slug, portsvc.ErrOutOfRange)
	}
	if err := h.service.ReleasePort(r.Context(), slug, port); err != nil {
		return h.portBack(w, r, slug, err)
	}
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.PortRelease,
		EntityType: "project",
		EntityID:   slug,
		Detail:     "port " + strconv.FormatInt(port, 10),
	})
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) ClaimPortPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
	}
	slug := r.PathValue("slug")
	port, err := strconv.Atoi(strings.TrimSpace(r.FormValue("port")))
	if err != nil {
		return h.portBack(w, r, slug, projectsvc.ErrInvalid)
	}
	if err := h.service.ClaimPort(r.Context(), slug, port); err != nil {
		return h.portBack(w, r, slug, err)
	}
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.PortClaim,
		EntityType: "project",
		EntityID:   slug,
		Detail:     "port " + strconv.Itoa(port),
	})
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) ReplaceMainPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
	}
	slug := r.PathValue("slug")
	claim := 0
	if raw := strings.TrimSpace(r.FormValue("port")); raw != "" {
		var err error
		claim, err = strconv.Atoi(raw)
		if err != nil {
			return h.portBack(w, r, slug, portsvc.ErrOutOfRange)
		}
	}
	releaseOld := r.FormValue("release_old") != "off"
	newPort, released, err := h.service.ReplaceMainPort(r.Context(), slug, claim, releaseOld)
	if err != nil {
		return h.portBack(w, r, slug, err)
	}
	detail := "port " + strconv.FormatInt(newPort, 10)
	if !released && releaseOld {
		detail += " (old main kept: still live)"
	}
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.PortPromote,
		EntityType: "project",
		EntityID:   slug,
		Detail:     detail,
	})
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) AddHealthcheckPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
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
	endpoint := r.FormValue("endpoint")
	if _, err := h.service.AddHealthcheck(r.Context(), slug, endpoint, expected); err != nil {
		return h.healthBack(w, r, slug, err)
	}
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.HealthAdd,
		EntityType: "project",
		EntityID:   slug,
		Detail:     endpoint,
	})
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) RemoveHealthcheckPost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
	}
	slug := r.PathValue("slug")
	if err := h.service.RemoveHealthcheck(r.Context(), slug, r.PathValue("id")); err != nil {
		return h.healthBack(w, r, slug, err)
	}
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.HealthRemove,
		EntityType: "healthcheck",
		EntityID:   r.PathValue("id"),
		Detail:     slug,
	})
	http.Redirect(w, r, portBackURL(r, slug), http.StatusSeeOther)
	return nil
}
