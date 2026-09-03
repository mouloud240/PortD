package layouts

func navClass(current, path string) string {
	if current == path {
		return "active"
	}
	return ""
}
