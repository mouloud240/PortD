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
	Title            string
	Path             string
	Action           string
	Error            string
	ID               string
	FullName         string
	Identifier       string
	Email            string
	Active           bool
	IsNew            bool
}
