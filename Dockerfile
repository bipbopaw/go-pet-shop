FROM golang:1.27.2-alpine3.24 AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o go-pet-shop ./cmd/app
RUN go build -o migrator ./cmd/migrator

FROM alpine:3.24.2

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/go-pet-shop ./go-pet-shop
COPY --from=builder /app/migrator ./migrator
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/config ./config

USER 10001:10001

EXPOSE 8080

CMD ["sh", "-c", "./migrator --migrations-path ./migrations && exec ./go-pet-shop"]

