FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /out/server ./server
COPY --from=build /src/web ./web
# orders.jsonl is written next to the binary at runtime — see README.md
# "Persistence" for why this needs a mounted volume in production.
RUN mkdir -p /app/data && chown appuser:appuser /app/data
USER appuser
ENV PORT=3000
ENV ORDERS_LOG_PATH=/app/data/orders.jsonl
EXPOSE 3000
ENTRYPOINT ["./server"]
