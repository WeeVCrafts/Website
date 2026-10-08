package legacy

import "strconv"

func legacyID(value int64) string { return strconv.FormatInt(value, 10) }

func legacyCents(value int64) string { return strconv.FormatInt(value, 10) }

func legacyPermissions(values []string) string {
	result := ""
	for i, value := range values {
		if i > 0 {
			result += ", "
		}
		result += value
	}
	return result
}
