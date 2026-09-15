package pages

import (
	"database/sql"
	"testing"

	"github.com/portd/internal/db/generated"
)

func TestActivityItemsHumanizesRows(t *testing.T) {
	t.Parallel()

	rows := []db.ActivityLog{
		{ID: "1", EventType: "project.create", EntityType: "project", EntityID: null("demo"), Outcome: "success", Detail: "Demo", CreatedAt: "2026-09-12T10:00:00Z"},
		{ID: "2", EventType: "runtime.start", EntityType: "project", EntityID: null("demo"), Outcome: "failure", Detail: "boom", CreatedAt: "2026-09-12T10:01:00Z", ActorInternID: null("intern-abcdef-1234")},
		{ID: "3", EventType: "auth.login", EntityType: "session", Outcome: "success", Detail: "username admin", CreatedAt: "not-a-time"},
		{ID: "4", EventType: "mystery.action_here", EntityType: "thing", Outcome: "success", CreatedAt: "2026-09-12T10:02:00Z"},
	}
	items := ActivityItems(rows)
	if len(items) != 4 {
		t.Fatalf("items = %d, want 4", len(items))
	}

	if items[0].EventLabel != "Projet créé" || items[0].Category != "Projets" || items[0].CategoryClass != "neutral" {
		t.Errorf("project item = %+v", items[0])
	}
	if items[1].EventLabel != "Exécution démarrée" || items[1].Category != "Exécution" || items[1].CategoryClass != "warning" {
		t.Errorf("runtime item = %+v", items[1])
	}
	if !items[1].Failed || items[1].OutcomeClass != "down" {
		t.Errorf("failed item outcome = %+v", items[1])
	}
	if items[1].ActorShort != "intern-a" || items[1].Actor != "intern-abcdef-1234" {
		t.Errorf("actor shortening = %+v", items[1])
	}
	if items[2].Actor != "admin" || items[2].EventLabel != "Connecté" || items[2].Category != "Auth" {
		t.Errorf("login item = %+v", items[2])
	}
	if items[2].Time != "not-a-time" {
		t.Errorf("unparsable time should pass through, got %q", items[2].Time)
	}
	if items[0].Time == "2026-09-12T10:00:00Z" {
		t.Errorf("parsable time should be reformatted, got %q", items[0].Time)
	}
	if items[3].EventLabel != "Action here" || items[3].Category != "Autre" {
		t.Errorf("unknown event fallback = %+v", items[3])
	}
	if got := CategoryName("port"); got != "Ports" {
		t.Errorf("CategoryName(port) = %q", got)
	}
}

func null(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}
