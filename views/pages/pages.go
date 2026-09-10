package pages

import "github.com/portd/internal/db/generated"

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
	Name        string
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

// DashboardData is the finished overview value; activity stays static for now.
type DashboardData struct {
	ActiveProjects int
	LiveServices   int
	TotalServices  int
	AssignedPorts  int
	UnknownPorts   int
	ActiveInterns  int
	Projects       []ProjectListItem
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
