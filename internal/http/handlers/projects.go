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
	"github.com/portd/internal/auth"
	db "github.com/portd/internal/db/generated"
	"github.com/portd/internal/httperr"
	portsvc "github.com/portd/internal/ports"
	projectsvc "github.com/portd/internal/projects"
	"github.com/portd/internal/runtime"
	"github.com/portd/views/pages"
)

func (h *ProjectsHandler) ListPage(w http.ResponseWriter, r *http.Request) error {
	search, lifecycle, liveParam, isLive := listFilters(r)
	principal, _ := auth.PrincipalFrom(r.Context())
	if !principal.IsAdmin() && principal.InternID == "" {
		return httperr.Forbidden("Connectez-vous en tant que stagiaire ou administrateur.", nil)
	}
	items, err := h.scopedList(r, search, lifecycle, isLive)
	if err != nil {
		if errors.Is(err, projectsvc.ErrInvalid) {
			return httperr.BadRequest("Filtre de projet invalide.", err)
		}
		return err
	}
	return httperr.Render(w, r, http.StatusOK, pages.ProjectsPage(
		"Projets",
		"/projects",
		search,
		lifecycle,
		liveParam,
		h.ProjectListItems(r.Context(), items),
		nil,
		false,
	))
}

// DetectPage scans disk and re-renders the projects page with the
// selection modal open. Nothing is registered until the modal posts back.
func (h *ProjectsHandler) DetectPage(w http.ResponseWriter, r *http.Request) error {
	search, lifecycle, liveParam, isLive := listFilters(r)
	principal, _ := auth.PrincipalFrom(r.Context())
	if !principal.IsAdmin() && principal.InternID == "" {
		return httperr.Forbidden("Connectez-vous en tant que stagiaire ou administrateur.", nil)
	}
	items, err := h.scopedList(r, search, lifecycle, isLive)
	if err != nil {
		if errors.Is(err, projectsvc.ErrInvalid) {
			return httperr.BadRequest("Filtre de projet invalide.", err)
		}
		return err
	}
	candidates, err := h.service.Scan(r.Context())
	if err != nil {
		return err
	}
	return httperr.Render(w, r, http.StatusOK, pages.ProjectsPage(
		"Projets",
		"/projects",
		search,
		lifecycle,
		liveParam,
		h.ProjectListItems(r.Context(), items),
		pages.DetectItems(candidates),
		true,
	))
}

func listFilters(r *http.Request) (search, lifecycle, liveParam string, isLive int64) {
	search = r.URL.Query().Get("q")
	lifecycle = r.URL.Query().Get("lifecycle_status")
	liveParam = r.URL.Query().Get("is_live")
	isLive = int64(-1)
	switch liveParam {
	case "true":
		isLive = 1
	case "false":
		isLive = 0
	default:
		liveParam = ""
	}
	return search, lifecycle, liveParam, isLive
}

func (h *ProjectsHandler) scopedList(r *http.Request, search, lifecycle string, isLive int64) ([]projectsvc.ProjectWithInterns, error) {
	principal, _ := auth.PrincipalFrom(r.Context())
	if principal.IsAdmin() {
		return h.service.List(r.Context(), search, lifecycle, isLive)
	}
	if principal.InternID == "" {
		return nil, projectsvc.ErrInvalid
	}
	return h.service.ListForIntern(r.Context(), principal.InternID, search, lifecycle, isLive)
}

func (h *ProjectsHandler) NewPage(w http.ResponseWriter, r *http.Request) error {
	selected := []string{}
	if principal, _ := auth.PrincipalFrom(r.Context()); !principal.IsAdmin() && principal.InternID != "" {
		selected = []string{principal.InternID}
	}
	options, err := h.internOptions(r, selected)
	if err != nil {
		return err
	}
	return httperr.Render(w, r, http.StatusOK, pages.ProjectFormPage(pages.ProjectFormData{
		Title:           "Nouveau projet",
		Path:            "/projects",
		Action:          "/projects",
		LifecycleStatus: "draft",
		PortCount:       2,
		IsNew:           true,
		AccessMode:      projectsvc.AccessModeDirect,
		Interns:         options,
	}))
}

func (h *ProjectsHandler) CreatePost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
	}
	shouldRun, _ := strconv.ParseBool(r.FormValue("should_run"))
	portCount := 2
	if raw := strings.TrimSpace(r.FormValue("ports_needed")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			portCount = n
		}
	}
	form := pages.ProjectFormData{
		Title:           "Nouveau projet",
		Path:            "/projects",
		Action:          "/projects",
		Name:            r.FormValue("name"),
		Slug:            r.FormValue("slug"),
		Description:     r.FormValue("description"),
		LifecycleStatus: r.FormValue("lifecycle_status"),
		ShouldRun:       shouldRun,
		PortCount:       portCount,
		IsNew:           true,
		AccessMode:      r.FormValue("access_mode"),
	}
	internIDs := r.Form["intern_ids"]
	options, err := h.internOptions(r, internIDs)
	if err != nil {
		return err
	}
	form.Interns = options

	creator := ""
	if principal, _ := auth.PrincipalFrom(r.Context()); !principal.IsAdmin() {
		creator = principal.InternID
	}
	created, err := h.service.Create(r.Context(), projectsvc.CreateInput{
		Name:            form.Name,
		Slug:            form.Slug,
		Description:     form.Description,
		InternIDs:       internIDs,
		LifecycleStatus: form.LifecycleStatus,
		ShouldRun:       form.ShouldRun,
		PortCount:       portCount,
		CreatorInternID: creator,
		AccessMode:      form.AccessMode,
	})
	if err != nil {
		slog.Error("project create failed", "error", err.Error())
		switch {
		case errors.Is(err, projectsvc.ErrInvalid):
			form.Error = "Le nom, au moins un stagiaire actif, un cycle de vie valide et 1 à 5 ports sont requis."
			return httperr.Render(w, r, http.StatusUnprocessableEntity, pages.ProjectFormPage(form))
		case errors.Is(err, projectsvc.ErrConflict):
			form.Error = "Un projet avec ce slug existe déjà."
			return httperr.Render(w, r, http.StatusConflict, pages.ProjectFormPage(form))
		case errors.Is(err, projectsvc.ErrScaffold):
			form.Error = "Le dossier du projet n'a pas pu être préparé. Vérifiez le dossier des projets et réessayez."
			return httperr.Render(w, r, http.StatusInternalServerError, pages.ProjectFormPage(form))
		case errors.Is(err, portsvc.ErrNoPorts):
			form.Error = "Pas assez de ports libres dans la plage 3000–9999 pour ce projet."
			return httperr.Render(w, r, http.StatusConflict, pages.ProjectFormPage(form))
		default:
			return err
		}
	}
	http.Redirect(w, r, "/projects/"+created.Project.Slug, http.StatusSeeOther)
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.ProjectCreate,
		EntityType: "project",
		EntityID:   created.Project.Slug,
		Detail:     created.Project.Name + " in " + created.Project.Directory,
	})
	return nil
}

func (h *ProjectsHandler) DetailPage(w http.ResponseWriter, r *http.Request) error {
	detail, err := h.service.GetBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		if errors.Is(err, projectsvc.ErrNotFound) {
			return httperr.NotFound("Projet introuvable.", err)
		}
		return err
	}
	data, err := h.projectDetailData(r.Context(), detail, r.URL.Query().Get("port_error"), r.URL.Query().Get("health_error"))
	if err != nil {
		return err
	}
	if message := r.URL.Query().Get("runtime_error"); message != "" {
		data.RuntimeError = message
	}
	return httperr.Render(w, r, http.StatusOK, pages.ProjectDetailPage(data))
}

func (h *ProjectsHandler) EditPage(w http.ResponseWriter, r *http.Request) error {
	detail, err := h.service.GetBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		if errors.Is(err, projectsvc.ErrNotFound) {
			return httperr.NotFound("Projet introuvable.", err)
		}
		return err
	}
	selected := make([]string, 0, len(detail.Interns))
	for _, intern := range detail.Interns {
		selected = append(selected, intern.ID)
	}
	options, err := h.internOptions(r, selected)
	if err != nil {
		return err
	}
	return httperr.Render(w, r, http.StatusOK, pages.ProjectFormPage(pages.ProjectFormData{
		Title:           "Modifier le projet",
		Path:            "/projects",
		Action:          "/projects/" + detail.Project.Slug,
		Name:            detail.Project.Name,
		Slug:            detail.Project.Slug,
		Description:     detail.Project.Description,
		LifecycleStatus: detail.Project.LifecycleStatus,
		ShouldRun:       detail.Project.ShouldRun == 1,
		IsLive:          detail.Project.IsLive == 1,
		AccessMode:      detail.Project.AccessMode,
		Ports:           h.assignedPorts(r.Context(), detail.Project.ID),
		PortError:       r.URL.Query().Get("port_error"),
		Interns:         options,
	}))
}

func (h *ProjectsHandler) UpdatePost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return httperr.BadRequest("Envoi de formulaire invalide.", err)
	}
	slug := r.PathValue("slug")
	shouldRun, _ := strconv.ParseBool(r.FormValue("should_run"))
	form := pages.ProjectFormData{
		Title:           "Modifier le projet",
		Path:            "/projects",
		Action:          "/projects/" + slug,
		Name:            r.FormValue("name"),
		Slug:            slug,
		Description:     r.FormValue("description"),
		LifecycleStatus: r.FormValue("lifecycle_status"),
		ShouldRun:       shouldRun,
		AccessMode:      r.FormValue("access_mode"),
	}
	internIDs := r.Form["intern_ids"]
	options, err := h.internOptions(r, internIDs)
	if err != nil {
		return err
	}
	form.Interns = options

	if projectID, err := h.service.GetBySlug(r.Context(), slug); err == nil {
		form.Ports = h.assignedPorts(r.Context(), projectID.Project.ID)
	}

	updated, err := h.service.Update(r.Context(), slug, projectsvc.UpdateInput{
		Name:            form.Name,
		Description:     form.Description,
		InternIDs:       internIDs,
		LifecycleStatus: form.LifecycleStatus,
		ShouldRun:       form.ShouldRun,
		AccessMode:      form.AccessMode,
	})
	if err != nil {
		switch {
		case errors.Is(err, projectsvc.ErrInvalid):
			form.Error = "Le nom, au moins un stagiaire actif et un cycle de vie valide sont requis."
			return httperr.Render(w, r, http.StatusUnprocessableEntity, pages.ProjectFormPage(form))
		case errors.Is(err, projectsvc.ErrNotFound):
			return httperr.NotFound("Projet introuvable.", err)
		default:
			return err
		}
	}
	form.IsLive = updated.Project.IsLive == 1
	http.Redirect(w, r, "/projects/"+updated.Project.Slug, http.StatusSeeOther)
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.ProjectUpdate,
		EntityType: "project",
		EntityID:   updated.Project.Slug,
		Detail:     updated.Project.Name,
	})
	return nil
}

func (h *ProjectsHandler) ArchivePost(w http.ResponseWriter, r *http.Request) error {
	slug := r.PathValue("slug")
	archived, err := h.service.Archive(r.Context(), slug)
	if err != nil {
		if errors.Is(err, projectsvc.ErrNotFound) {
			return httperr.NotFound("Projet introuvable.", err)
		}
		return err
	}
	http.Redirect(w, r, "/projects/"+archived.Project.Slug, http.StatusSeeOther)
	h.activity.Record(r.Context(), activitysvc.Event{
		EventType:  activitysvc.ProjectArchive,
		EntityType: "project",
		EntityID:   archived.Project.Slug,
		Detail:     archived.Project.Name,
	})
	return nil
}

func (h *ProjectsHandler) healthBack(w http.ResponseWriter, r *http.Request, slug string, err error) error {
	if errors.Is(err, projectsvc.ErrNotFound) {
		return httperr.NotFound("Projet introuvable.", err)
	}
	msg := "L'action sur le contrôle a échoué."
	if errors.Is(err, projectsvc.ErrInvalid) {
		msg = "Le point de terminaison doit commencer par / ou http(s):// et attendre un statut 200–599."
	} else {
		return err
	}
	next := portBackURL(r, slug)
	sep := "?"
	if strings.Contains(next, "?") {
		sep = "&"
	}
	http.Redirect(w, r, next+sep+"health_error="+url.QueryEscape(msg), http.StatusSeeOther)
	return nil
}

func parsePort(raw string) (int64, bool) {
	port, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || port < 1 || port > 65535 {
		return 0, false
	}
	return port, true
}

func portBackURL(r *http.Request, slug string) string {
	if next := r.FormValue("next"); strings.HasPrefix(next, "/projects/") {
		return next
	}
	return "/projects/" + slug
}

func (h *ProjectsHandler) portBack(w http.ResponseWriter, r *http.Request, slug string, err error) error {
	if errors.Is(err, projectsvc.ErrNotFound) || errors.Is(err, portsvc.ErrNotFound) {
		return httperr.NotFound("Projet introuvable.", err)
	}
	msg := "L'action sur le port a échoué."
	switch {
	case errors.Is(err, projectsvc.ErrInvalid):
		msg = "Le nombre doit être entre 1 et 5."
	case errors.Is(err, portsvc.ErrNoPorts):
		msg = "Pas assez de ports libres dans la plage 3000–9999."
	case errors.Is(err, portsvc.ErrPortTaken):
		msg = "Ce port est déjà pris."
	case errors.Is(err, portsvc.ErrOutOfRange):
		msg = "Le port doit être entre 3000 et 9999."
	case errors.Is(err, portsvc.ErrMainPort):
		msg = "Promeuvez un autre port comme principal avant de libérer celui-ci."
	case errors.Is(err, portsvc.ErrPortLive):
		msg = "Ce port est utilisé. Arrêtez d'abord le processus."
	default:
		return err
	}
	next := portBackURL(r, slug)
	sep := "?"
	if strings.Contains(next, "?") {
		sep = "&"
	}
	http.Redirect(w, r, next+sep+"port_error="+url.QueryEscape(msg), http.StatusSeeOther)
	return nil
}

func (h *ProjectsHandler) internOptions(r *http.Request, selected []string) ([]pages.InternOption, error) {
	interns, err := h.service.ListActiveInterns(r.Context())
	if err != nil {
		return nil, err
	}
	selectedSet := make(map[string]struct{}, len(selected))
	for _, id := range selected {
		selectedSet[id] = struct{}{}
	}
	options := make([]pages.InternOption, 0, len(interns))
	for _, intern := range interns {
		_, ok := selectedSet[intern.ID]
		options = append(options, pages.InternOption{
			ID:       intern.ID,
			FullName: intern.FullName,
			Selected: ok,
		})
	}
	return options, nil
}

func (h *ProjectsHandler) ProjectListItems(ctx context.Context, items []projectsvc.ProjectWithInterns) []pages.ProjectListItem {
	out := make([]pages.ProjectListItem, 0, len(items))
	for _, item := range items {
		names := make([]string, 0, len(item.Interns))
		for _, intern := range item.Interns {
			names = append(names, intern.FullName)
		}
		shouldRun := item.Project.ShouldRun == 1
		isLive := item.Project.IsLive == 1
		statusLabel, statusClass := pages.StatusBadge(shouldRun, isLive)
		access := h.projectAccess(ctx, item.Project)
		mainPort := "—"
		if main, err := h.service.MainPort(ctx, item.Project.ID); err == nil {
			mainPort = strconv.FormatInt(main.Port, 10)
		}
		out = append(out, pages.ProjectListItem{
			Name:            item.Project.Name,
			Slug:            item.Project.Slug,
			Interns:         strings.Join(names, ", "),
			StatusLabel:     statusLabel,
			StatusClass:     statusClass,
			LifecycleStatus: item.Project.LifecycleStatus,
			LifecycleLabel:  pages.LifecycleLabel(item.Project.LifecycleStatus),
			LifecycleClass:  pages.LifecycleClass(item.Project.LifecycleStatus),
			MainPort:        mainPort,
			URL:             access.URL,
			URLLabel:        access.URLLabel,
			AccessMode:      access.Mode,
			AccessLabel:     access.ModeLabel,
			DirectURL:       access.DirectURL,
			DirectURLLabel:  access.DirectURLLabel,
			ProxiedURL:      access.ProxiedURL,
			ProxiedURLLabel: access.ProxiedURLLabel,
			UpdatedAt:       formatUpdatedAt(item.Project.UpdatedAt),
			MissingStartup:  projectsvc.MissingStartup(item.Project.Directory),
		})
	}
	return out
}

func (h *ProjectsHandler) projectDetailData(ctx context.Context, detail projectsvc.ProjectWithInterns, portError, healthError string) (pages.ProjectDetailData, error) {
	names := make([]string, 0, len(detail.Interns))
	for _, intern := range detail.Interns {
		names = append(names, intern.FullName)
	}
	owners := strings.Join(names, ", ")
	if owners == "" {
		owners = "—"
	}
	shouldRun := detail.Project.ShouldRun == 1
	isLive := detail.Project.IsLive == 1
	statusLabel, statusClass := pages.StatusBadge(shouldRun, isLive)
	access := h.projectAccess(ctx, detail.Project)
	runtimeIntent := "Arrêt demandé"
	if shouldRun {
		runtimeIntent = "Doit tourner"
	}
	runtimeStatus := h.runtime.Status(detail.Project.ID)
	runtimeFile := runtimeStatus.File
	if runtimeFile == "" {
		runtimeFile = detail.Project.StartupCommand
	}
	runtimeState := string(runtimeStatus.State)
	if runtimeState == "" {
		runtimeState = string(runtime.StateStopped)
	}
	runtimeError := ""
	if runtimeStatus.Error != "" {
		runtimeError = runtimeStatus.Error
	}
	if runtimeState == string(runtime.StateRunning) {
		statusLabel, statusClass = "En cours", "running"
	} else if runtimeState == string(runtime.StateFailed) {
		statusLabel, statusClass = "Échoué", "failed"
	}
	ports := h.assignedPorts(ctx, detail.Project.ID)
	mainPort := "—"
	for _, item := range ports {
		if item.IsMain {
			mainPort = item.Port
		}
	}
	checks, err := h.service.ListHealthchecks(ctx, detail.Project.ID)
	if err != nil {
		return pages.ProjectDetailData{}, err
	}
	endpoints := make([]pages.HealthcheckItem, 0, len(checks))
	for _, check := range checks {
		endpoints = append(endpoints, pages.HealthcheckItem{
			ID:       check.ID,
			Endpoint: check.Endpoint,
			Expected: strconv.FormatInt(check.ExpectedStatus, 10),
		})
	}
	return pages.ProjectDetailData{
		Path:            "/projects",
		Name:            detail.Project.Name,
		Slug:            detail.Project.Slug,
		Description:     detail.Project.Description,
		Owners:          owners,
		CreatedAt:       formatUpdatedAt(detail.Project.CreatedAt),
		UpdatedAt:       formatUpdatedAt(detail.Project.UpdatedAt),
		Directory:       detail.Project.Directory,
		StartupCommand:  detail.Project.StartupCommand,
		LifecycleStatus: detail.Project.LifecycleStatus,
		LifecycleLabel:  pages.LifecycleLabel(detail.Project.LifecycleStatus),
		LifecycleClass:  pages.LifecycleClass(detail.Project.LifecycleStatus),
		LifecyclePhase:  pages.LifecyclePhase(detail.Project.LifecycleStatus),
		RuntimeIntent:   runtimeIntent,
		MissingStartup:  projectsvc.MissingStartup(detail.Project.Directory),
		RuntimeState:    runtimeState,
		RuntimePID:      strconv.Itoa(runtimeStatus.PID),
		RuntimeFile:     runtimeFile,
		RuntimeError:    runtimeError,
		ShouldRun:       shouldRun,
		IsLive:          isLive,
		StatusLabel:     statusLabel,
		StatusClass:     statusClass,
		URL:             access.URL,
		URLLabel:        access.URLLabel,
		Access:          access,
		MainPort:        mainPort,
		AllocatedPorts:  ports,
		PortError:       portError,
		Healthchecks:    endpoints,
		HealthError:     healthError,
		Archived:        detail.Project.LifecycleStatus == "archived",
	}, nil
}

func (h *ProjectsHandler) projectAccess(ctx context.Context, project db.Project) pages.ProjectAccessData {
	main, err := h.service.MainPort(ctx, project.ID)
	if err != nil {
		return pages.ProjectAccessData{Mode: project.AccessMode, ModeLabel: accessModeLabel(project.AccessMode)}
	}
	direct, err := h.service.DirectProjectURL(main.Port)
	if err != nil {
		direct = ""
	}
	proxied := h.service.ProjectURL(project.Slug)
	access := pages.ProjectAccessData{
		Mode:            project.AccessMode,
		ModeLabel:       accessModeLabel(project.AccessMode),
		DirectURL:       direct,
		DirectURLLabel:  urlLabel(direct),
		ProxiedURL:      proxied,
		ProxiedURLLabel: urlLabel(proxied),
		Quickstarts:     quickstarts(proxied),
		AIPrompt:        aiPrompt(proxied, direct),
	}

	access.URL, access.URLLabel = access.ProxiedURL, access.ProxiedURLLabel
	if project.AccessMode == projectsvc.AccessModeDirect {
		access.URL, access.URLLabel = access.DirectURL, access.DirectURLLabel
	}
	return access
}

func aiPrompt(proxied, direct string) string {
	return "PortD est un centre de projets interne qui propose deux façons d'ouvrir les applications existantes :\n" +
		"- Mode proxy : " + proxied + " — une URL partagée propre sous PortD, qui peut exiger un chemin de base du framework.\n" +
		"- Mode direct : " + direct + " — le serveur de l'application et son port principal, sans préfixe PortD ni modification requise.\n\n" +
		"Mon application est en cours de préparation pour le mode proxy. Identifie le framework et configure son chemin de base pour que " +
		"le routage côté client, les ressources à chemin absolu, les redirections, les formulaires, les appels API et les liens générés côté serveur fonctionnent sous l'URL proxy. " +
		"Ne réécris pas de code sans rapport et ne casse pas le mode direct. Dis-moi exactement quel fichier modifier, explique pourquoi, et fournis le plus petit correctif sûr. " +
		"Si cette application ne peut pas supporter un préfixe de chemin de façon fiable, dis-le et recommande plutôt le mode direct."
}

func accessModeLabel(mode string) string {
	if mode == projectsvc.AccessModeProxied {
		return "Via proxy"
	}
	return "Direct"
}

func quickstarts(proxied string) []pages.QuickstartItem {
	base := proxied
	if parsed, err := url.Parse(proxied); err == nil && parsed.Path != "" {
		base = strings.TrimRight(parsed.Path, "/")
	}
	return []pages.QuickstartItem{
		{Key: "react-router", Name: "React Router", File: "src/main.jsx ou src/main.tsx", Description: "Passez le chemin du projet comme basename du routeur.", Snippet: `<BrowserRouter basename="` + base + `">`},
		{Key: "vite", Name: "Vite", File: "vite.config.js ou vite.config.ts", Description: "Définissez la base pour résoudre les ressources sous le chemin du projet.", Snippet: "base: '" + base + "/'"},
		{Key: "next", Name: "Next.js", File: "next.config.js ou next.config.mjs", Description: "Définissez basePath une fois pour les liens et les ressources.", Snippet: "basePath: '" + base + "'"},
		{Key: "vue-router", Name: "Vue Router", File: "src/router/index.js ou src/router/index.ts", Description: "Passez le chemin du projet au mode historique.", Snippet: "createWebHistory('" + base + "/')"},
		{Key: "nuxt", Name: "Nuxt", File: "nuxt.config.ts ou nuxt.config.js", Description: "Définissez baseURL pour le routage et les ressources.", Snippet: "app: { baseURL: '" + base + "/' }"},
		{Key: "angular", Name: "Angular", File: "src/index.html", Description: "Définissez la base du document pour le routeur et les URL.", Snippet: `<base href="` + base + `/">`},
		{Key: "sveltekit", Name: "SvelteKit", File: "svelte.config.js", Description: "Définissez le chemin de base de l'adaptateur.", Snippet: "paths: { base: '" + base + "' }"},
		{Key: "hash-routing", Name: "Routage par hash", File: "Aucun fichier à modifier", Description: "Les routes par hash fonctionnent déjà sous un préfixe de chemin.", Snippet: "Aucune configuration requise", NoConfig: true},
	}
}

func (h *ProjectsHandler) assignedPorts(ctx context.Context, projectID string) []pages.PortItem {
	rows, err := h.service.ListPorts(ctx, projectID)
	if err != nil {
		return nil
	}
	live := map[int64]bool{}
	if observed, err := h.service.ListPortObservations(ctx); err == nil {
		for _, row := range observed {
			live[row.Port] = true
		}
	}
	items := make([]pages.PortItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, pages.PortItem{
			Port:   strconv.FormatInt(row.Port, 10),
			Role:   row.Role,
			IsMain: row.Role == "main",
			Live:   live[row.Port],
		})
	}
	return items
}

func urlLabel(raw string) string {
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	return raw
}

func formatUpdatedAt(value string) string {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC().Format("2006-01-02 15:04")
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC().Format("2006-01-02 15:04")
	}
	return value
}
