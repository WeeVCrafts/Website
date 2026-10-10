.PHONY: templ-generate templ-check css-build css-check web-test seed-demo

TEMPL_VERSION := v0.3.1020

templ-generate:
	go run github.com/a-h/templ/cmd/templ@$(TEMPL_VERSION) generate ./web/...

templ-check:
	go run github.com/a-h/templ/cmd/templ@$(TEMPL_VERSION) generate --check ./web/...

css-build:
	npm run css:build

css-check:
	@test -s internal/transport/web/static/app.css
	@test -s web/assets/css/fonts.min.css
	@test -s web/assets/css/theme.min.css
	@test -s web/assets/css/utility.min.css
	@test -s web/assets/css/customer-polish.min.css
	@test -s web/assets/css/admin.min.css
	@test -s web/assets/css/seller-admin.min.css
	@test -s web/assets/css/support-portal.min.css

web-test:
	go test ./internal/web/... ./web/...

seed-demo:
	go run ./cmd/seed-demo
