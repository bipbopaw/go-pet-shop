# Go Pet Shop

Учебный REST API интернет-магазина товаров для питомцев на Go и PostgreSQL. Реализованы регистрация и просмотр пользователей, каталог товаров, CRUD-операции для товаров, создание заказов и добавление в них позиций.

## Текущие маршруты

| Метод | Путь | Действие |
| --- | --- | --- |
| `GET` | `/health` | Проверка доступности HTTP-сервера |
| `POST` | `/users` | Создать пользователя |
| `GET` | `/users` | Получить список пользователей |
| `GET` | `/users/{email}` | Найти пользователя по email |
| `POST` | `/orders` | Создать заказ |
| `POST` | `/orders/{id}/items` | Добавить позицию в заказ |
| `GET` | `/orders/{id}` | Получить заказ с позициями и данными товаров |
| `GET` | `/users/{email}/orders` | Получить заказы пользователя |
| `GET` | `/products/{id}` | Получить товар по ID |
| `GET` | `/products` | Получить список товаров |
| `POST` | `/products` | Создать товар |
| `PUT` | `/products/{id}` | Обновить товар |
| `DELETE` | `/products/{id}` | Удалить товар |

Для создания заказа передайте `user_id` и `total_price`, например `{"user_id":1,"total_price":25.50}`. Чтобы добавить позицию, передайте `product_id` и `quantity`, например `{"product_id":2,"quantity":3}`. ID позиции возвращается в ответе.

Для списков пользователей и товаров поддерживаются query-параметры `limit` и `offset`, например `/products?limit=10&offset=20`. По умолчанию возвращается до 20 записей; допустимый `limit` — от 1 до 100. JSON-тела запросов ограничены 1 МиБ.

## Структура

```text
cmd/app/                    запуск HTTP-сервера
cmd/migrator/               запуск миграций
config/                     конфигурация окружений
internal/config/            загрузка конфигурации
internal/handlers/          HTTP-обработчики
internal/lib/httpx/         разбор параметров пагинации
internal/lib/logger/        настройка логирования
internal/models/            структуры данных
internal/storage/           ошибки хранилища
internal/storage/postgres/  PostgreSQL-хранилище
migrations/                 SQL-миграции
```

## Запуск локально

Нужны Go, Task и PostgreSQL. Создайте `.env` со строкой подключения, например:

```dotenv
DATABASE_URL=postgres://user:password@localhost:5432/petshop?sslmode=disable
```

Примените миграции и запустите сервер:

```sh
task migrate
go run ./cmd/app
```

По умолчанию сервер слушает `localhost:8080`. Выбор файла конфигурации задаётся переменной `APP_ENV` (`local` по умолчанию; доступен также `dev`). Строка подключения обязательна в `DATABASE_URL`.

Для запуска через Docker Compose задайте в `.env` значения `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` и `DATABASE_URL`. Внутри Compose адрес базы в `DATABASE_URL` должен указывать на хост `postgres`, например `postgres://user:password@postgres:5432/petshop?sslmode=disable`.

## Миграции и линтер

```sh
task migrate
task linter
```
