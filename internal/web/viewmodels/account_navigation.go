package viewmodels

// AccountDestination is shared by the desktop profile menu and mobile drawer.
// The server supplies a role-owned route; older renderers retain /account.
func (p CustomerPage) AccountDestination() string {
	switch p.AccountURL {
	case "/seller-admin/settings", "/admin", "/super-admin":
		return p.AccountURL
	default:
		return "/account"
	}
}
