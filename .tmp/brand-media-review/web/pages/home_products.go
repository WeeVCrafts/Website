package pages

func categoryCurrent(selected bool) string {
	if selected {
		return "true"
	}
	return "false"
}
