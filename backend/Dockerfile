FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

FROM alpine:3.22

WORKDIR /app
RUN apk add --no-cache ca-certificates wget

COPY --from=build /out/api /app/api
COPY --from=build /src/migrations /app/migrations

ENV PORT=8080

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:${PORT:-8080}/health || exit 1

CMD ["/app/api"]
