package interns

import (
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/portd/internal/db/generated"
)

var templateFuncs = template.FuncMap{"active": func(current, path string) string {
	if current == path || (path == "/interns" && len(current) >= len(path) && current[:len(path)] == path) {
		return "active"
	}
	return ""
}}

var internListTemplate = template.Must(template.New("shell.html").Funcs(templateFuncs).ParseFiles(viewPath("views/layouts/shell.html"), viewPath("views/pages/interns.html")))
var internFormTemplate = template.Must(template.New("shell.html").Funcs(templateFuncs).ParseFiles(viewPath("views/layouts/shell.html"), viewPath("views/pages/intern_form.html")))

type pageData struct {
	Title, Search, Error string
	Path                 string
	Items                []db.Intern
	Intern               db.Intern
	New                  bool
}

func (s *Service) ListPage(w http.ResponseWriter, r *http.Request) error {
	items, err := s.List(r.Context(), r.URL.Query().Get("q"), -1)
	if err != nil {
		return err
	}
	return render(w, internListTemplate, "interns", pageData{Title: "Interns", Path: r.URL.Path, Search: r.URL.Query().Get("q"), Items: items})
}
func (s *Service) NewPage(w http.ResponseWriter, _ *http.Request) error {
	return render(w, internFormTemplate, "intern_form", pageData{Title: "New intern", Path: "/interns", New: true})
}
func (s *Service) CreatePost(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}
	intern, err := s.Create(r.Context(), r.FormValue("full_name"), r.FormValue("email"), r.FormValue("identifier"), r.FormValue("password"))
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return render(w, internFormTemplate, "intern_form", pageData{Title: "New intern", Path: "/interns", New: true, Error: "Full name, username, and password are required."})
	}
	http.Redirect(w, r, "/interns/"+intern.ID, http.StatusSeeOther)
	return nil
}
func (s *Service) DetailPage(w http.ResponseWriter, r *http.Request) error {
	intern, err := s.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	return render(w, internFormTemplate, "intern_form", pageData{Title: "Edit intern", Path: "/interns", Intern: intern})
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
func render(w http.ResponseWriter, templates *template.Template, name string, data pageData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return templates.ExecuteTemplate(w, name, data)
}

func viewPath(path string) string {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return filepath.Join(root, path)
}
