package interns

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/portd/internal/db/generated"
)

const shell = `<script src="https://cdn.tailwindcss.com"></script><script>tailwind.config={theme:{extend:{colors:{ink:'#172b4d',blue:'#003da5',green:'#087f44',ground:'#f5f8fb'}}}}</script>`

var page = template.Must(template.New("interns").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Interns · PortD</title>` + shell + `</head><body class="min-h-screen bg-ground text-slate-700"><div class="flex min-h-screen"><aside class="hidden w-64 shrink-0 border-r border-slate-200 bg-white p-5 md:flex md:flex-col"><a class="mb-7 px-2 text-xl font-extrabold tracking-tight text-ink" href="/">Project Hub</a><nav class="space-y-1 text-sm font-semibold"><a class="block rounded-md bg-green/10 px-3 py-2 text-green" href="/">Overview</a><p class="px-3 pb-1 pt-5 text-xs font-bold uppercase tracking-wider text-slate-400">Projects</p><a class="block rounded-md px-3 py-2 hover:bg-slate-100" href="/projects">All projects</a><a class="block rounded-md px-3 py-2 hover:bg-slate-100" href="/projects/tracking">Project tracking</a><a class="block rounded-md px-3 py-2 hover:bg-slate-100" href="/projects/new">Add project</a><p class="px-3 pb-1 pt-5 text-xs font-bold uppercase tracking-wider text-slate-400">Infrastructure</p><a class="block rounded-md px-3 py-2 hover:bg-slate-100" href="/ports">Port table</a><p class="px-3 pb-1 pt-5 text-xs font-bold uppercase tracking-wider text-slate-400">People</p><a class="block rounded-md bg-green/10 px-3 py-2 text-green" href="/interns">Interns</a></nav><p class="mt-auto border-t border-slate-200 px-2 pt-4 text-xs text-slate-400">Internal use only</p></aside><main class="min-w-0 flex-1"><header class="flex h-16 items-center justify-between border-b border-slate-200 bg-white px-5 md:px-8"><div><strong class="text-sm text-ink">People</strong><span class="ml-3 text-sm font-semibold text-green">● System online</span></div><form method="post" action="/logout"><button class="rounded-md border border-slate-300 px-3 py-2 text-sm font-bold text-slate-700 hover:bg-slate-50">Sign out</button></form></header><div class="mx-auto max-w-6xl p-5 md:p-8"><div class="mb-6 flex flex-col justify-between gap-4 sm:flex-row sm:items-start"><div><h1 class="text-2xl font-extrabold tracking-tight text-ink">Interns</h1><p class="mt-1 text-sm text-slate-500">Create intern accounts and see the projects they own.</p></div><a class="rounded-md bg-blue px-4 py-2 text-sm font-bold text-white hover:bg-blue/90" href="/interns/new">+ Add intern</a></div><section class="overflow-hidden rounded-md border border-slate-200 bg-white"><form class="flex gap-3 border-b border-slate-200 p-4" role="search"><input class="min-w-0 flex-1 rounded border border-slate-300 px-3 py-2 text-sm outline-none focus:border-blue focus:ring-2 focus:ring-blue/20" name="q" value="{{.Search}}" placeholder="Search interns" aria-label="Search interns"><button class="rounded border border-slate-300 px-4 py-2 text-sm font-bold hover:bg-slate-50">Search</button></form><div class="overflow-x-auto"><table class="w-full text-left text-sm"><thead class="bg-slate-50 text-xs uppercase tracking-wide text-slate-500"><tr><th class="px-5 py-3">Name</th><th class="px-5 py-3">Username</th><th class="px-5 py-3">Email</th><th class="px-5 py-3">State</th></tr></thead><tbody class="divide-y divide-slate-100">{{range .Items}}<tr class="hover:bg-slate-50"><td class="px-5 py-3 font-bold text-ink"><a class="text-blue hover:underline" href="/interns/{{.ID}}">{{.FullName}}</a></td><td class="px-5 py-3 font-mono text-xs">{{.Identifier.String}}</td><td class="px-5 py-3">{{.Email.String}}</td><td class="px-5 py-3">{{if .Active}}<span class="rounded-full bg-green/10 px-2 py-1 text-xs font-bold text-green">Active</span>{{else}}<span class="rounded-full bg-slate-100 px-2 py-1 text-xs font-bold text-slate-500">Inactive</span>{{end}}</td></tr>{{else}}<tr><td class="px-5 py-10 text-center text-slate-500" colspan="4">No interns found.</td></tr>{{end}}</tbody></table></div></section></div></main></div></body></html>`))
var form = template.Must(template.New("intern-form").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}} · PortD</title>` + shell + `</head><body class="min-h-screen bg-ground text-slate-700"><div class="flex min-h-screen"><aside class="hidden w-64 shrink-0 border-r border-slate-200 bg-white p-5 md:flex md:flex-col"><a class="mb-7 px-2 text-xl font-extrabold tracking-tight text-ink" href="/">Project Hub</a><nav class="space-y-1 text-sm font-semibold"><a class="block rounded-md px-3 py-2 hover:bg-slate-100" href="/">Overview</a><p class="px-3 pb-1 pt-5 text-xs font-bold uppercase tracking-wider text-slate-400">Projects</p><a class="block rounded-md px-3 py-2 hover:bg-slate-100" href="/projects">All projects</a><a class="block rounded-md px-3 py-2 hover:bg-slate-100" href="/projects/tracking">Project tracking</a><a class="block rounded-md px-3 py-2 hover:bg-slate-100" href="/projects/new">Add project</a><p class="px-3 pb-1 pt-5 text-xs font-bold uppercase tracking-wider text-slate-400">Infrastructure</p><a class="block rounded-md px-3 py-2 hover:bg-slate-100" href="/ports">Port table</a><p class="px-3 pb-1 pt-5 text-xs font-bold uppercase tracking-wider text-slate-400">People</p><a class="block rounded-md bg-green/10 px-3 py-2 text-green" href="/interns">Interns</a></nav></aside><main class="min-w-0 flex-1"><header class="flex h-16 items-center justify-end border-b border-slate-200 bg-white px-5 md:px-8"><form method="post" action="/logout"><button class="rounded-md border border-slate-300 px-3 py-2 text-sm font-bold">Sign out</button></form></header><div class="mx-auto max-w-3xl p-5 md:p-8"><a class="text-sm font-semibold text-blue hover:underline" href="/interns">← Interns</a><h1 class="mt-5 text-2xl font-extrabold tracking-tight text-ink">{{.Title}}</h1>{{if .Error}}<p class="mt-4 rounded border border-red-200 bg-red-50 p-3 text-sm font-semibold text-red-700" role="alert">{{.Error}}</p>{{end}}<form class="mt-6 space-y-5 rounded-md border border-slate-200 bg-white p-5 md:p-7" method="post"><label class="block text-sm font-bold text-slate-700">Full name<input class="mt-2 w-full rounded border border-slate-300 px-3 py-2 font-normal outline-none focus:border-blue focus:ring-2 focus:ring-blue/20" name="full_name" required value="{{.Intern.FullName}}"></label><label class="block text-sm font-bold text-slate-700">Username<input class="mt-2 w-full rounded border border-slate-300 px-3 py-2 font-normal outline-none focus:border-blue focus:ring-2 focus:ring-blue/20" name="identifier" required value="{{.Intern.Identifier.String}}"></label><label class="block text-sm font-bold text-slate-700">Email<input class="mt-2 w-full rounded border border-slate-300 px-3 py-2 font-normal outline-none focus:border-blue focus:ring-2 focus:ring-blue/20" type="email" name="email" value="{{.Intern.Email.String}}"></label>{{if .New}}<label class="block text-sm font-bold text-slate-700">Password<input class="mt-2 w-full rounded border border-slate-300 px-3 py-2 font-normal outline-none focus:border-blue focus:ring-2 focus:ring-blue/20" type="password" name="password" required></label>{{end}}<button class="rounded-md bg-blue px-4 py-2 text-sm font-bold text-white hover:bg-blue/90">Save intern</button></form></div></main></div></body></html>`))

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
