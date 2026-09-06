# UI Architecture (current: templ-only, post-migration)

No `html/template` remains (grep finds zero matches). All HTML is
`templ` components; all handlers render via `httperr.Render`.
Generated `*_templ.go` files are committed.

## 1. View tree

```
views/layouts/layout.templ  (package layouts, Shell(title, path, body))
views/layouts/layout.go     (navClass(current, path) → active-link classes)
views/layouts/layout_templ.go        (generated, committed)
views/components/fields.templ (package components, PasswordField(name, autocomplete, required))
views/components/fields_templ.go     (generated, committed)
views/pages/pages.templ     (package pages: DashboardPage, PlaceholderPage,
                             LoginPage(errorMessage), InternsPage,
                             InternFormPage + private bodies)
views/pages/pages.go        (view models: InternListItem, ListItems() constructor,
                             InternFormData)
views/pages/pages_templ.go           (generated, committed)
```

- `Shell` owns doctype/head/css+JS/sidebar/header; pages pass
  `(title, path, bodyComponent)`. `LoginPage` is standalone (no `Shell`).
- View models keep SQL out of templates: `ListItems([]db.Intern)` flattens
  `sql.NullString`/`Active int` once; `InternFormData` carries every field
  including redisplay values plus `Error`/`IsNew`.
- `PasswordField` owns the show/hide toggle via Alpine
  (`x-data="{ show: false }"`, `:type`, `@click`); Go owns all other values.

## 2. Render + error pipeline (`internal/httperr/httperr.go`)

- `Handler` = `func(w, r) error`; `Handle()` wraps it into `http.Handler`,
  writing the error only if nothing was written yet (tracked writer).
- `Render(w, r, status, component)` renders to a buffer first, then writes
  headers+body — all-or-nothing, partial output never reaches the client.
- Constructors: `BadRequest` (400), `Unauthorized` (401), `Forbidden` (403),
  `NotFound` (404), `Conflict` (409). Unknown errors → 500 via `writeError`
  (plain HTML stub, cause logged, message not leaked).

## 3. Route → handler → component map (`internal/app/app.go`)

| Route | Handler | Component |
|---|---|---|
| `GET /`, `GET /dashboard` | `app.dashboard` | `pages.DashboardPage()` |
| `GET /projects`, `/projects/tracking`, `/projects/new`, `/ports`, `/activity` | `app.placeholder(title)` | `pages.PlaceholderPage(title, r.URL.Path)` |
| `GET /login` | `auth.LoginPage` | `pages.LoginPage("")` |
| `POST /login` | `auth.LoginPost` | `pages.LoginPage(msg)` on 401, else redirect `/dashboard` (303) |
| `POST /logout` | `auth.LogoutPost` | redirect `/login` (303) |
| `GET /profile` | `app.profilePage` | `pages.ProfilePage(ProfileData)` (own record, editable for interns; read-only for admin) |
| `POST /profile` | `app.profileUpdate` | re-render `ProfilePage` on 422/409, else redirect `/profile` (303); admins redirect (303) unchanged |
| `GET /interns` | `interns.ListPage` | `pages.InternsPage("Interns", path, q, ListItems(rows))` |
| `GET /interns/new` | `interns.NewPage` | `pages.InternFormPage(New InternFormData{IsNew:true})` |
| `POST /interns` | `interns.CreatePost` | re-render `InternFormPage` on 422/409, else redirect `/interns/{id}` (303) |
| `GET /interns/{id}` | `interns.DetailPage` | `pages.InternFormPage(...)` or 404 |
| `POST /interns/{id}` | `interns.UpdatePost` | re-render on 422/409, 404 if gone, else redirect (303) |

Guards: `requireSession` (→ `/login` 303), `requireAdmin` (+ 403 for
non-admins). Static: `GET /static/` → `web/static`. Health: `GET /healthz`.

## 4. JS + CSS

- Alpine.js + alpine-ajax are pinned local files in `web/static`
  (`alpine.min.js`, `alpine-ajax.min.js`), loaded with `defer` in
  `Shell` and `LoginPage`. No CDN.
- Usages: form busy flag (`x-data="{ busy: false }"`, `@submit="busy = true"`,
  `:disabled="busy"` on save button in `internFormBody`); search form
  (`method="get" x-target="intern-table"` on `#intern-table` in `internsBody`);
  password toggle (see §1).
- Tailwind: `web/input.css` (`@tailwind base/components/utilities`) +
  `tailwind.config.js` (content scans `views/**/*.{html,templ}`,
  `internal/**/*.{go,html}`; `brand.*` palette, Inter/mono fonts) →
  `npm run build:css` writes committed bundle `web/static/app.css`.

## 5. Add a new page

1. View model (if needed): add struct/constructor in `views/pages/pages.go`.
2. Component: add `templ XxxPage(...)` (+ private `xxxBody`) in
   `views/pages/pages.templ`, wrapped in `@layouts.Shell(title, path, body)`
   (or standalone like `LoginPage` if no chrome wanted).
3. Handler: add `func (s *Service) Xxx(...) error` returning
   `httperr.Render(w, r, http.StatusOK, pages.XxxPage(...))`; wire route in
   `internal/app/app.go` with `httperr.Handle(...)` (+ `requireSession`/
   `requireAdmin` as needed).
4. Regenerate + format + rebuild CSS (see §6).

## 6. Required commands after edits

```sh
templ generate   # regenerate *_templ.go after any .templ change
templ fmt        # keep .templ formatting canonical
npm run build:css  # rebuild web/static/app.css after class/markup changes
```

## 7. Error-status conventions

| Case | Status | Behavior |
|---|---|---|
| Validation failure (empty name/user, bad input) | 422 | re-render same form with `form.Error` set, input preserved from `r.FormValue` |
| Duplicate username/email (`ErrConflict`) | 409 | re-render same form, `"That username or email is already taken."` |
| Unknown intern id (`ErrNotFound` / `sql.ErrNoRows`) | 404 | `httperr.NotFound("Intern not found.", err)` |
| Unparseable form (`r.ParseForm` fails) | 400 | `httperr.BadRequest("Invalid form submission.", err)` |
| Bad credentials | 401 | re-render `LoginPage("Invalid username or password.")` |
| Non-admin on admin route | 403 | `httperr.Forbidden(...)` |
| Success POST | 303 | `http.Redirect(..., StatusSeeOther)` to detail/dashboard page |
