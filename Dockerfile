FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum* ./
# Railway requires cache IDs scoped to the Website service.
RUN --mount=type=cache,id=s/6a3550f0-1e4f-4e1a-98dc-4de16951e669-/go/pkg/mod,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,id=s/6a3550f0-1e4f-4e1a-98dc-4de16951e669-/go/pkg/mod,target=/go/pkg/mod --mount=type=cache,id=s/6a3550f0-1e4f-4e1a-98dc-4de16951e669-/root/.cache/go-build,target=/root/.cache/go-build CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/wecratfs ./cmd/website && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/weevcrafts-sync-accounts ./cmd/sync-designated-accounts

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
RUN addgroup -S app && adduser -S -D -H -G app app
WORKDIR /app
RUN mkdir -p /app/web/assets/images/uploads && chown -R app:app /app
COPY --from=build /out/wecratfs /usr/local/bin/wecratfs
COPY --from=build /out/weevcrafts-sync-accounts /usr/local/bin/weevcrafts-sync-accounts
EXPOSE 8080
USER app
ENTRYPOINT ["/usr/local/bin/wecratfs"]
