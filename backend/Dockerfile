FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

FROM alpine:3.22

WORKDIR /app
RUN apk add --no-cache wget

COPY --from=build /out/api /app/api
COPY --from=build /src/migrations /app/migrations

EXPOSE 8080

CMD ["/app/api"]
