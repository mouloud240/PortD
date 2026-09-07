package ports

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	db "github.com/portd/internal/db/generated"
	"github.com/portd/internal/httperr"
	"github.com/portd/views/pages"
)

func (s *Service) ListPage(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.Snapshot(r.Context())
	if err != nil {
		return err
	}
	items := make([]pages.PortRow, 0, len(rows))
	unknown := 0
	for _, row := range rows {
		item := pages.PortRow{Port: strconv.FormatInt(row.Port, 10), Process: displayProcess(row), Interns: "—"}
		if row.ProjectID.Valid {
			item.ProjectID = row.ProjectID.String
			item.Registered = true
			s.enrich(r, &item)
		} else {
			unknown++
		}
		items = append(items, item)
	}
	return httperr.Render(w, r, http.StatusOK, pages.PortsPage(pages.PortsPageData{
		Path:      r.URL.Path,
		CheckedAt: time.Now().Format("15:04:05"),
		Unknown:   unknown,
		Rows:      items,
	}))
}

func displayProcess(row db.PortObservation) string {
	switch {
	case row.ProcessName.Valid && row.ProcessID.Valid:
		return row.ProcessName.String + " (" + strconv.FormatInt(row.ProcessID.Int64, 10) + ")"
	case row.ProcessName.Valid:
		return row.ProcessName.String
	case row.ProcessID.Valid:
		return "PID " + strconv.FormatInt(row.ProcessID.Int64, 10)
	default:
		return "—"
	}
}

func (s *Service) enrich(r *http.Request, item *pages.PortRow) {
	project, err := s.queries.GetProjectByID(r.Context(), item.ProjectID)
	if err != nil {
		item.Project = "—"
		return
	}
	item.Project = project.Name
	item.ProjectSlug = project.Slug
	interns, err := s.queries.ListProjectInterns(r.Context(), item.ProjectID)
	if err != nil {
		return
	}
	names := make([]string, 0, len(interns))
	for _, intern := range interns {
		names = append(names, intern.FullName)
	}
	item.Interns = strings.Join(names, ", ")
}
