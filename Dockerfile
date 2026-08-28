# Single image containing both binaries. Which one runs is picked by the
# command, not the build target — needed so Fly.io (and similar "process
# groups" platforms) can run "app" and "worker" as two processes from one
# built image (see fly.toml [processes]). docker-compose picks the command
# per service instead of building two separate images.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server
RUN CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /out/server ./server
COPY --from=build /out/worker ./worker
COPY --from=build /src/migrations ./migrations
COPY --from=build /src/web ./web
USER appuser
EXPOSE 3000
ENTRYPOINT ["./server"]
