# url-shortener

REST-сервис для коротких ссылок на Go + PostgreSQL.

**Стек:** Go 1.25, chi, pgx (PostgreSQL 17), slog, Docker Compose, testify, httpexpect.

## Запуск

Нужен только Docker.

```bash
git clone https://github.com/bpinr/url-shortener.git
cd url-shortener
cp .env.example .env
docker compose up
```

Всё. Postgres, миграции и сервис поднимаются одной командой: сначала Postgres, затем одноразовый контейнер `migrate` накатывает SQL-миграции из `migrations/`, и только после этого стартует сервис.

| Что        | Где / значение |
|------------|----------------|
| Сервис     | `http://localhost:8082` |
| Basic Auth | `admin` / `admin` (`AUTH_USER` / `AUTH_PASSWORD` в `.env`) |
| PostgreSQL | `postgres:5432` внутри сети compose, БД `url_shortener`, юзер `shortener`, пароль `shortener` (`POSTGRES_*` в `.env`) |

Порт Postgres наружу не пробрасывается — поэтому запуск не упадёт, даже если на машине уже работает свой Postgres на 5432. Зайти в базу:

```bash
docker compose exec postgres psql -U shortener -d url_shortener
```

Остановить: `docker compose down` (вместе с данными: `docker compose down -v`).
После изменений в коде: `docker compose up --build`.

## Миграции

Схема БД живёт в `migrations/` в формате [golang-migrate](https://github.com/golang-migrate/migrate): пара `NNNNNN_<name>.up.sql` / `.down.sql` на каждое изменение. Сам сервис схему не создаёт и не меняет.

Новая миграция — следующий номер (`000002_<name>.up.sql` + `.down.sql`), применится при следующем `docker compose up`. Откатить последнюю:

```bash
docker compose run --rm migrate -path=/migrations \
  -database="postgres://shortener:shortener@postgres:5432/url_shortener?sslmode=disable" down 1
```

## API

`POST`, `PUT`, `DELETE` закрыты Basic Auth. Редирект — публичный.

| Метод    | Путь           | Тело | Ответ |
|----------|----------------|------|-------|
| `POST`   | `/url`         | `{"url": "https://go.dev", "alias": "go"}` (`alias` необязателен — сгенерируется) | `201` `{"status":"OK","alias":"go"}` |
| `GET`    | `/{alias}`     | — | `302` редирект на оригинальный URL |
| `PUT`    | `/url/{alias}` | `{"url": "https://pkg.go.dev"}` | `200` `{"status":"OK","alias":"go","url":"https://pkg.go.dev"}` |
| `DELETE` | `/url/{alias}` | — | `200` `{"status":"OK"}` |

### Коды ошибок

| Код   | Когда |
|-------|-------|
| `400` | битый JSON, невалидный URL (нужен абсолютный `http`/`https`), невалидный alias (`a-z A-Z 0-9 _ -`, до 64 символов) |
| `401` | нет Basic Auth или неверные креды |
| `404` | alias не найден (редирект, обновление, удаление) |
| `409` | alias уже занят |
| `500` | внутренняя ошибка (например, БД недоступна) |

Ошибки приходят в виде `{"status":"Error","error":"<описание>"}`.

### Примеры

```bash
# создать
curl -i -u admin:admin -X POST localhost:8082/url \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://go.dev","alias":"go"}'

# перейти
curl -i localhost:8082/go

# обновить
curl -i -u admin:admin -X PUT localhost:8082/url/go \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://pkg.go.dev"}'

# удалить
curl -i -u admin:admin -X DELETE localhost:8082/url/go
```

## Тесты

Пока в соседнем терминале работает `docker compose up`:

```bash
go test ./...
```

- **Unit** — `internal/service` (бизнес-логика на in-memory хранилище) и `internal/http-server/handlers/...` (HTTP-статусы и ответы, service подменён стабом). Отдельная функция на каждый кейс: `TestSave_Success`, `TestSave_BadRequest`, `TestSave_Conflict`, `TestUpdate_NotFound`, `TestDelete_Success` и т.д.
- **E2E** — `tests/`, ходят в живой сервис на `localhost:8082`: 201/400/401/404/409, update, delete. Креды Basic Auth читаются из того же `.env`, что и у сервиса. Если сервис не запущен, e2e-тесты пропускаются (`SKIP`), unit-тесты работают без Docker.

## Архитектура

```
handler  →  service  →  storage
(HTTP)      (логика)    (SQL)
```

- **handler** (`internal/http-server/handlers`) — парсит запрос, вызывает service, по ошибке service выбирает HTTP-статус. Бизнес-логики нет.
- **service** (`internal/service`) — валидация URL и alias, генерация alias, обработка занятого alias (с повтором для сгенерированных), перевод ошибок storage в ошибки сервиса.
- **storage** (`internal/storage/postgres`) — только SQL. Уникальность alias гарантирует `UNIQUE` в таблице.

Все ручки, включая редирект, идут через service.

```
cmd/url-shortener/       точка входа, сборка слоёв, graceful shutdown
config/local.yaml        несекретные настройки (порт, таймауты, длина alias)
internal/config/         загрузка конфига: YAML + переменные окружения
internal/http-server/    хендлеры и middleware
internal/service/        бизнес-логика
internal/storage/        ошибки хранилища и реализация на Postgres
migrations/              SQL-миграции схемы БД (golang-migrate)
tests/                   e2e-тесты
```

## Конфигурация

- `config/local.yaml` — несекретное: окружение, адрес и таймауты HTTP, длина alias.
- `.env` — секреты и подключение к БД: `POSTGRES_*`, `AUTH_USER`, `AUTH_PASSWORD`. Его читают и docker compose, и сервис, и e2e-тесты. В репозитории только `.env.example`, сам `.env` в `.gitignore`.

## Что сделано сверх видео-гайда

Основа (chi, slog, Basic Auth, тесты на httpexpect) — по видео-гайду. Сверх него:

- PostgreSQL (pgx) вместо SQLite, запуск одной командой через Docker Compose
- слой service между handler и storage — вся бизнес-логика там, все ручки (включая редирект) идут через него
- ручка обновления `PUT /url/{alias}`
- HTTP-статусы по смыслу: 201 / 400 / 401 / 404 / 409 / 500
- отдельная тест-функция на каждый кейс, тесты на update и delete, e2e-тесты с кредами из `.env`
- graceful shutdown
