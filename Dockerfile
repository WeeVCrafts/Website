FROM node:24-alpine AS assets
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY scripts ./scripts
COPY web ./web
COPY internal ./internal
COPY tailwind.config.js ./
RUN npm run assets:images && npm run css:build

FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum* ./
# Railway requires cache IDs scoped to the Website service.
RUN --mount=type=cache,id=s/6a3550f0-1e4f-4e1a-98dc-4de16951e669-/go/pkg/mod,target=/go/pkg/mod go mod download
RUN --mount=type=cache,id=s/6a3550f0-1e4f-4e1a-98dc-4de16951e669-/go/pkg/mod,target=/go/pkg/mod --mount=type=cache,id=s/6a3550f0-1e4f-4e1a-98dc-4de16951e669-/root/.cache/go-build,target=/root/.cache/go-build go install github.com/a-h/templ/cmd/templ@v0.3.1020
COPY . .
COPY --from=assets /src/web/assets ./web/assets
COPY --from=assets /src/web/components/image_variants_generated.go ./web/components/image_variants_generated.go
COPY --from=assets /src/internal/transport/web/static/app.css ./internal/transport/web/static/app.css
RUN templ generate -path . -w 2
RUN --mount=type=cache,id=s/6a3550f0-1e4f-4e1a-98dc-4de16951e669-/go/pkg/mod,target=/go/pkg/mod --mount=type=cache,id=s/6a3550f0-1e4f-4e1a-98dc-4de16951e669-/root/.cache/go-build,target=/root/.cache/go-build CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/wecratfs ./cmd/website && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/weevcrafts-sync-accounts ./cmd/sync-designated-accounts

FROM alpine:3.22
LABEL com.weevcrafts.project="weevcrafts" com.weevcrafts.role="application"
RUN apk add --no-cache ca-certificates
RUN addgroup -S app && adduser -S -D -H -G app app
WORKDIR /app
RUN mkdir -p /app/web/assets/images/uploads && chown -R app:app /app
COPY --from=build /out/wecratfs /usr/local/bin/wecratfs
COPY --from=build /out/weevcrafts-sync-accounts /usr/local/bin/weevcrafts-sync-accounts
EXPOSE 8080
USER app
ENTRYPOINT ["/usr/local/bin/wecratfs"]
