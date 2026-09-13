package pages

import (
	"strings"
	"time"

	"github.com/portd/internal/db/generated"
	"github.com/portd/internal/projects"
)

// InternListItem is the finished row value the list template ranges over.
type InternListItem struct {
	ID       string
	FullName string
	Username string
	Email    string
	Active   bool
}

// ListItems converts query rows once so templates never touch sql.NullString.
func ListItems(interns []db.Intern) []InternListItem {
	items := make([]InternListItem, 0, len(interns))
	for _, intern := range interns {
		items = append(items, InternListItem{
			ID:       intern.ID,
			FullName: intern.FullName,
			Username: intern.Identifier.String,
			Email:    intern.Email.String,
			Active:   intern.Active == 1,
		})
	}
	return items
}

// InternFormData is the finished form value; handlers fill every field,
// including the values to redisplay after a validation failure.
// Action is the explicit POST target: without it the browser posts back to
// the current URL, so the new-intern form would hit POST /interns/{id} with
// id="new" instead of creating anything.
type InternFormData struct {
	Title      string
	Path       string
	Action     string
	Error      string
	ID         string
	FullName   string
	Identifier string
	Email      string
	Active     bool
	IsNew      bool
}

// ProfileData is the finished profile value; admins get a read-only view,
// interns get their own record for editing.
type ProfileData struct {
	Title    string
	Path     string
	Role     string
	IsAdmin  bool
	FullName string
	Username string
	Email    string
	Error    string
}

// ProjectListItem is one row on the projects directory.
type ProjectListItem struct {
	Name            string
	Slug            string
	Interns         string
	StatusLabel     string
	StatusClass     string
	LifecycleStatus string
	LifecycleLabel  string
	LifecycleClass  string
	MainPort        string
	URL             string
	URLLabel        string
	AccessMode      string
	AccessLabel     string
	DirectURL       string
	DirectURLLabel  string
	ProxiedURL      string
	ProxiedURLLabel string
	UpdatedAt       string
	MissingStartup  bool
}

// DetectItem is one disk folder in the detect selection modal.
type DetectItem struct {
	Name      string
	Slug      string
	Directory string
	Exists    bool
}

// DetectItems converts scan candidates once so templates stay logic-free.
func DetectItems(candidates []projects.DetectedCandidate) []DetectItem {
	items := make([]DetectItem, 0, len(candidates))
	for _, c := range candidates {
		items = append(items, DetectItem{
			Name:      c.Name,
			Slug:      c.Slug,
			Directory: c.Directory,
			Exists:    c.Exists,
		})
	}
	return items
}

// InternOption is a selectable intern on the project form.
type InternOption struct {
	ID       string
	FullName string
	Selected bool
}

// ProjectFormData is the finished project create/edit form value.
type ProjectFormData struct {
	Title           string
	Path            string
	Action          string
	Error           string
	Name            string
	Slug            string
	Description     string
	LifecycleStatus string
	ShouldRun       bool
	PortCount       int
	Ports           []PortItem
	PortError       string
	IsLive          bool
	IsNew           bool
	AccessMode      string
	Interns         []InternOption
}

// PortItem is one assigned port on project pages.
type PortItem struct {
	Port   string
	Role   string
	IsMain bool
	Live   bool
}

// HealthcheckItem is one stored healthcheck endpoint on the detail page.
type HealthcheckItem struct {
	ID       string
	Endpoint string
	Expected string
}

type QuickstartItem struct {
	Key         string
	Name        string
	File        string
	Description string
	Snippet     string
	NoConfig    bool
}

type ProjectAccessData struct {
	Mode            string
	ModeLabel       string
	URL             string
	URLLabel        string
	DirectURL       string
	DirectURLLabel  string
	ProxiedURL      string
	ProxiedURLLabel string
	Quickstarts     []QuickstartItem
	AIPrompt        string
}

// ProjectDetailData is the finished project detail page value.
type ProjectDetailData struct {
	Path            string
	Name            string
	Slug            string
	Description     string
	Owners          string
	CreatedAt       string
	UpdatedAt       string
	Directory       string
	StartupCommand  string
	LifecycleStatus string
	LifecycleLabel  string
	LifecycleClass  string
	LifecyclePhase  string
	RuntimeIntent   string
	MissingStartup  bool
	RuntimeState    string
	RuntimePID      string
	RuntimeFile     string
	RuntimeError    string
	ShouldRun       bool
	IsLive          bool
	StatusLabel     string
	StatusClass     string
	URL             string
	URLLabel        string
	MainPort        string
	AllocatedPorts  []PortItem
	PortError       string
	Healthchecks    []HealthcheckItem
	HealthError     string
	Archived        bool
	Access          ProjectAccessData
}

// PortRow is one observed listening port, enriched for display.
// ProjectID empty means the port is not registered to any project.
type PortRow struct {
	Port        string
	Process     string
	ProjectID   string
	Project     string
	ProjectSlug string
	Interns     string
	Registered  bool
}

// PortsPageData is the finished port table value.
type PortsPageData struct {
	Path      string
	CheckedAt string
	Unknown   int
	Rows      []PortRow
}

// LifecycleBar is one bar of the overview lifecycle breakdown.
type LifecycleBar struct {
	Status  string
	Label   string
	Class   string
	Color   string
	Count   int
	Percent int
}

// DashboardData is the finished overview value.
type DashboardData struct {
	ActiveProjects int
	LiveServices   int
	TotalServices  int
	AssignedPorts  int
	UnknownPorts   int
	ActiveInterns  int
	Lifecycle      []LifecycleBar
	Projects       []ProjectListItem
	Recent         []ActivityItem
}

// ActivityItem is one audit row with display-ready strings.
type ActivityItem struct {
	Time          string
	Event         string
	EventLabel    string
	Category      string
	CategoryClass string
	Entity        string
	Actor         string
	ActorShort    string
	Outcome       string
	OutcomeClass  string
	Detail        string
	Failed        bool
}

// ActivityData is the finished audit listing value.
type ActivityData struct {
	Title      string
	Path       string
	Items      []ActivityItem
	EventTypes []string
	EventType  string
	Categories []string
	Category   string
	Outcome    string
	Search     string
	Page       int
	HasPrev    bool
	HasNext    bool
}

// ActivityItems converts log rows once so templates never touch sql.NullString.
func ActivityItems(rows []db.ActivityLog) []ActivityItem {
	items := make([]ActivityItem, 0, len(rows))
	for _, row := range rows {
		entity := row.EntityType
		if row.EntityID.Valid {
			entity += ":" + row.EntityID.String
		}
		actor := "admin"
		actorShort := "admin"
		if row.ActorInternID.Valid {
			actor = row.ActorInternID.String
			actorShort = actor
			if len(actorShort) > 8 {
				actorShort = actorShort[:8]
			}
		}
		class := "running"
		if row.Outcome != "success" {
			class = "down"
		}
		category, categoryClass := activityCategory(row.EventType)
		items = append(items, ActivityItem{
			Time:          activityTime(row.CreatedAt),
			Event:         row.EventType,
			EventLabel:    activityLabel(row.EventType),
			Category:      category,
			CategoryClass: categoryClass,
			Entity:        entity,
			Actor:         actor,
			ActorShort:    actorShort,
			Outcome:       row.Outcome,
			OutcomeClass:  class,
			Detail:        row.Detail,
			Failed:        row.Outcome != "success",
		})
	}
	return items
}

func activityTime(ts string) string {
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t.Local().Format("Jan 2 15:04")
	}
	return ts
}

func activityLiClass(failed bool) string {
	if failed {
		return "red"
	}
	return ""
}

// activityCategory derives the display category from the event prefix.
func activityCategory(event string) (string, string) {
	switch {
	case strings.HasPrefix(event, "project."):
		return "Projects", "neutral"
	case strings.HasPrefix(event, "runtime."):
		return "Runtime", "warning"
	case strings.HasPrefix(event, "port."):
		return "Ports", "info"
	case strings.HasPrefix(event, "intern."):
		return "Interns", "running"
	case strings.HasPrefix(event, "auth."):
		return "Auth", "accent"
	case strings.HasPrefix(event, "healthcheck."):
		return "Healthchecks", "info"
	default:
		return "Other", "neutral"
	}
}

// CategoryName turns a filter prefix into its display name.
func CategoryName(prefix string) string {
	name, _ := activityCategory(prefix + ".x")
	return name
}

// activityLabel turns an event code into a human sentence fragment.
func activityLabel(event string) string {
	switch event {
	case "auth.login":
		return "Signed in"
	case "auth.logout":
		return "Signed out"
	case "project.create":
		return "Project created"
	case "project.update":
		return "Project updated"
	case "project.archive":
		return "Project archived"
	case "runtime.start":
		return "Runtime started"
	case "runtime.stop":
		return "Runtime stopped"
	case "runtime.configure":
		return "Startup file saved"
	case "port.allocate":
		return "Ports allocated"
	case "port.release":
		return "Port released"
	case "port.claim":
		return "Port claimed"
	case "port.promote":
		return "Port promoted"
	case "healthcheck.add":
		return "Healthcheck added"
	case "healthcheck.remove":
		return "Healthcheck removed"
	case "intern.create":
		return "Intern added"
	case "intern.update":
		return "Intern updated"
	case "intern.profile":
		return "Profile updated"
	default:
		action := event
		if i := strings.LastIndex(action, "."); i >= 0 {
			action = action[i+1:]
		}
		action = strings.ReplaceAll(action, "_", " ")
		if action == "" {
			return event
		}
		return strings.ToUpper(action[:1]) + action[1:]
	}
}

// activeJS renders a Go bool as a JS boolean literal for x-init.
func activeJS(active bool) string {
	if active {
		return "true"
	}
	return "false"
}

func portCountOrDefault(n int) int {
	if n < 1 || n > 5 {
		return 2
	}
	return n
}

func lifecycleLabel(status string) string {
	switch status {
	case "draft":
		return "Draft"
	case "ready":
		return "Ready"
	case "running":
		return "Running"
	case "stopped":
		return "Stopped"
	case "failed":
		return "Failed"
	case "archived":
		return "Archived"
	default:
		return status
	}
}

// LifecycleLabel returns the display label for a lifecycle status.
func LifecycleLabel(status string) string { return lifecycleLabel(status) }

func lifecyclePhase(status string) string {
	switch status {
	case "draft":
		return "Define scope and owners"
	case "ready":
		return "Ready for first deploy"
	case "running":
		return "Active development runtime"
	case "stopped":
		return "Paused — restart when needed"
	case "failed":
		return "Investigate runtime failure"
	case "archived":
		return "Cleanup handoff"
	default:
		return "Current phase"
	}
}

// LifecyclePhase is the short tracking phase copy for a lifecycle status.
func LifecyclePhase(status string) string { return lifecyclePhase(status) }

func lifecycleClass(status string) string {
	switch status {
	case "draft", "ready", "running", "stopped", "failed", "archived":
		return status
	default:
		return "archived"
	}
}

// LifecycleClass returns the prototype state-pill modifier for a lifecycle.
func LifecycleClass(status string) string { return lifecycleClass(status) }

func statusBadge(shouldRun, isLive bool) (label, class string) {
	switch {
	case isLive:
		return "Running", "running"
	case shouldRun:
		return "Attention", "warning"
	default:
		return "Down", "down"
	}
}

// StatusBadge returns the prototype reachability badge label and class.
func StatusBadge(shouldRun, isLive bool) (label, class string) {
	return statusBadge(shouldRun, isLive)
}
