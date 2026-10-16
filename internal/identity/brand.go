package identity

import "strings"

const (
	Name        = "WeevCrafts"
	Tagline     = "Handmade by India’s Makers"
	Description = "Shop handmade Indian arts, crafts, sarees, home decor and gifts from independent makers at WeevCrafts."
	HomeTitle   = "WeevCrafts | Handmade Indian Arts, Crafts & Sarees"
)

var brandDisplayNames = strings.NewReplacer("WeeVCrafts", Name, "Weevcrafts", Name)

// DisplayName corrects platform branding in presentation without rewriting
// stored account identities, catalog facts, credentials or historical records.
func DisplayName(value string) string {
	return brandDisplayNames.Replace(value)
}
