package layouts

func navClass(current, path string) string {
	if current == path {
		return "bg-brand-green-soft text-brand-green font-bold"
	}
	return "text-slate-700 hover:bg-slate-100"
}
