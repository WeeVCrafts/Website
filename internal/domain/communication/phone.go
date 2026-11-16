package communication

import "strings"

// IndianMobile accepts common Indian display formats and returns international
// digits. It never substitutes a delivery recipient for the account contact.
func IndianMobile(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 24 {
		return "", false
	}
	var b strings.Builder
	for i, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
			continue
		}
		if r == '+' && i == 0 || r == ' ' || r == '-' || r == '(' || r == ')' {
			continue
		}
		return "", false
	}
	n := b.String()
	if len(n) == 11 && n[0] == '0' {
		n = n[1:]
	}
	if len(n) == 12 && strings.HasPrefix(n, "91") {
		n = n[2:]
	}
	if len(n) != 10 || n[0] < '6' || n[0] > '9' {
		return "", false
	}
	return "91" + n, true
}
