package layouts

func navClass(current, path string) string {
	if current == path {
		return "bg-brand-green-soft text-brand-green font-bold dark:bg-green-950 dark:text-green-300"
	}
	return "text-slate-700 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-700"
}
