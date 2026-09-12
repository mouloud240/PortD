package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	activitysvc "github.com/portd/internal/activity"
	"github.com/portd/internal/httperr"
	"github.com/portd/views/pages"
)

// activityPageSize is the audit listing page size.
const activityPageSize = 50

// activityCategories feeds the listing filter; each value is an event prefix.
var activityCategories = []string{"project", "runtime", "port", "intern", "auth", "healthcheck"}

func validCategory(category string) string {
	for _, known := range activityCategories {
		if category == known {
			return category
		}
	}
	return ""
}

// activityEventTypes feeds the listing filter in event recording order.
var activityEventTypes = []string{
	activitysvc.AuthLogin,
	activitysvc.AuthLogout,
	activitysvc.ProjectCreate,
	activitysvc.ProjectUpdate,
	activitysvc.ProjectArchive,
	activitysvc.RuntimeStart,
	activitysvc.RuntimeStop,
	activitysvc.RuntimeConfigure,
	activitysvc.PortAllocate,
	activitysvc.PortRelease,
	activitysvc.PortClaim,
	activitysvc.PortPromote,
	activitysvc.HealthAdd,
	activitysvc.HealthRemove,
	activitysvc.InternCreate,
	activitysvc.InternUpdate,
	activitysvc.ProfileUpdate,
}

func (h *Handlers) Activity(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query()
	page := 1
	if n, err := strconv.Atoi(query.Get("page")); err == nil && n > 0 {
		page = n
	}
	eventType := query.Get("event_type")
	category := validCategory(query.Get("category"))
	outcome := query.Get("outcome")
	if outcome != "" && outcome != activitysvc.OutcomeSuccess && outcome != activitysvc.OutcomeFailure {
		outcome = ""
	}
	rows, err := h.activity.List(r.Context(), activitysvc.Filter{
		EventType: eventType,
		Category:  category,
		Outcome:   outcome,
		Search:    query.Get("q"),
		Limit:     activityPageSize + 1,
		Offset:    int64((page - 1) * activityPageSize),
	})
	if err != nil {
		return err
	}
	hasNext := len(rows) > activityPageSize
	if hasNext {
		rows = rows[:activityPageSize]
	}
	return httperr.Render(w, r, http.StatusOK, pages.ActivityPage(pages.ActivityData{
		Title:      "Activity",
		Path:       "/activity",
		Items:      pages.ActivityItems(rows),
		EventTypes: activityEventTypes,
		EventType:  eventType,
		Categories: activityCategories,
		Category:   category,
		Outcome:    outcome,
		Search:     query.Get("q"),
		Page:       page,
		HasPrev:    page > 1,
		HasNext:    hasNext,
	}))
}

// recentActivity loads the dashboard feed; audit failures never fail the page.
func (h *Handlers) recentActivity(r *http.Request) []pages.ActivityItem {
	rows, err := h.activity.Recent(r.Context(), 8)
	if err != nil {
		slog.Error("activity feed failed", "error", err)
		return nil
	}
	return pages.ActivityItems(rows)
}
