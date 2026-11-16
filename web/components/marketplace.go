package components

import "strings"

func marketplaceQueueLabel(value string) string {
	if value == "" {
		return "All exceptions"
	}
	return strings.ReplaceAll(value, "_", " ")
}
