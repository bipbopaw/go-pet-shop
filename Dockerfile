# Этап сборки
FROM golang:1.27.2-alpine3.24 AS builder

WORKDIR /app

# Устанавливаем зависимости для сборки
RUN apk add --no-cache git

# Копируем go.mod и go.sum отдельно для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект
COPY . .

# Собираем бинарник
RUN go build -o go-pet-shop ./cmd/app
# Собираем бинарник для миграций
RUN go build -o migrator ./cmd/migrator

# Финальный минимальный образ
FROM alpine:3.24.2

WORKDIR /app

# Устанавливаем сертификаты (нужны для работы HTTP-клиентов)
RUN apk --no-cache add ca-certificates

COPY --from=builder /app/go-pet-shop ./go-pet-shop
# Бинарник который умеет выполнять миграции
COPY --from=builder /app/migrator ./migrator
# Копируем файлы миграции
COPY --from=builder /app/migrations ./migrations
# Прокидываем config
COPY --from=builder /app/config ./config

USER 10001:10001

EXPOSE 8080

CMD ["sh", "-c", "./migrator --migrations-path ./migrations && exec ./go-pet-shop"]

