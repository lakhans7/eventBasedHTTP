# Multi-stage build. Produces two small images from one Dockerfile via the
# --target flag: "server" (the API) and "worker" (the asynq job processor).
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server
RUN CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

FROM alpine:3.20 AS server
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /out/server ./server
COPY --from=build /src/migrations ./migrations
COPY --from=build /src/web ./web
USER appuser
EXPOSE 3000
ENTRYPOINT ["./server"]

FROM alpine:3.20 AS worker
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /out/worker ./worker
USER appuser
ENTRYPOINT ["./worker"]
