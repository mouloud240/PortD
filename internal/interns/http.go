package interns

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/portd/internal/db/generated"
)

var page = template.Must(template.New("interns").Parse(`<!doctype html><html><head><meta charset="utf-8"><title>Interns · PortD</title></head><body><main><h1>Interns</h1><p><a href="/interns/new">Add intern</a></p><form><input name="q" value="{{.Search}}" placeholder="Search interns"><button>Search</button></form><table><tr><th>Name</th><th>Username</th><th>Email</th><th>State</th></tr>{{range .Items}}<tr><td><a href="/interns/{{.ID}}">{{.FullName}}</a></td><td>{{.Identifier.String}}</td><td>{{.Email.String}}</td><td>{{if .Active}}Active{{else}}Inactive{{end}}</td></tr>{{else}}<tr><td colspan="4">No interns found.</td></tr>{{end}}</table></main></body></html>`))
var form = template.Must(template.New("intern-form").Parse(`<!doctype html><html><head><meta charset="utf-8"><title>{{.Title}} · PortD</title></head><body><main><h1>{{.Title}}</h1>{{if .Error}}<p role="alert">{{.Error}}</p>{{end}}<form method="post"><label>Full name <input name="full_name" required value="{{.Intern.FullName}}"></label><label>Username <input name="identifier" required value="{{.Intern.Identifier.String}}"></label><label>Email <input type="email" name="email" value="{{.Intern.Email.String}}"></label>{{if .New}}<label>Password <input type="password" name="password" required></label>{{end}}<button>Save</button></form></main></body></html>`))

func (s *Service) ListPage(w http.ResponseWriter, r *http.Request) error {
	items, err := s.List(r.Context(), r.URL.Query().Get("q"), -1)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return page.Execute(w, struct {
		Items  []db.Intern
		Search string
	}{items, r.URL.Query().Get("q")})
}
func (s *Service) NewPage(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return form.Execute(w, struct {
		Title  string
		Intern db.Intern
		New    bool
		Error  string
	}{"New intern", db.Intern{}, true, ""})
}
func (s *Service) CreatePost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}
	intern, err := s.Create(r.Context(), r.FormValue("full_name"), r.FormValue("email"), r.FormValue("identifier"), r.FormValue("password"))
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return form.Execute(w, struct {
			Title  string
			Intern db.Intern
			New    bool
			Error  string
		}{"New intern", db.Intern{}, true, "Full name, username, and password are required."})
	}
	http.Redirect(w, r, "/interns/"+intern.ID, http.StatusSeeOther)
	return nil
}
func (s *Service) DetailPage(w http.ResponseWriter, r *http.Request) error {
	intern, err := s.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return form.Execute(w, struct {
		Title  string
		Intern db.Intern
		New    bool
		Error  string
	}{"Edit intern", intern, false, ""})
}
func (s *Service) UpdatePost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}
	active, _ := strconv.ParseBool(r.FormValue("active"))
	intern, err := s.Update(r.Context(), r.PathValue("id"), r.FormValue("full_name"), r.FormValue("email"), r.FormValue("identifier"), active)
	if err != nil {
		return err
	}
	http.Redirect(w, r, "/interns/"+intern.ID, http.StatusSeeOther)
	return nil
}
