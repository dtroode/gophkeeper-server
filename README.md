# GophKeeper Server

Серверная часть менеджера паролей GophKeeper - gRPC сервер для безопасного хранения и синхронизации приватных данных пользователей.

Сервер реализует полную бизнес-логику для регистрации и аутентификации пользователей, защищённого хранения данных с использованием PostgreSQL и MinIO, а также синхронизации между множественными клиентами одного пользователя. Все данные шифруются на стороне клиента, сервер работает только с зашифрованными данными и никогда не имеет доступа к приватной информации пользователей.

## Для разработчиков

### Установка зависимостей
```bash
go mod download
```

### Генерация моков
```bash
make mocks
# или с помощью Docker
docker run --rm -e GOTOOLCHAIN=auto -v "$PWD":/src -w /src vektra/mockery
```

### Тестирование
```bash
# Юнит-тесты
make test
go test ./... -count=1 -cover

# Покрытие кода
make cover
go test ./... -count=1 -coverprofile=profiles.coverage.out && \
    go tool cover -func=profiles.coverage.out | tail -n 1

# Интеграционные тесты
make test-integration
go test -tags=integration ./internal/repository/postgres -v -count=1
```

### Сборка с версионной информацией
```bash
# Сборка с автоматическими версионными флагами
go build -ldflags "-X main.buildVersion=$(git describe --tags --always --dirty) -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.buildCommit=$(git rev-parse --short HEAD)" -o gophkeeper-server ./cmd

# Проверка версии
./gophkeeper-server --version # покажет версию при запуске
```

### Миграции базы данных
```bash
# Применить миграции
export DATABASE_DSN="postgres://user:password@localhost/gophkeeper?sslmode=disable"
cd database/migrations
goose postgres $DATABASE_DSN up

# Откат миграций
goose postgres $DATABASE_DSN down
```

### Переменные окружения для разработки
```bash
export DATABASE_DSN="postgres://user:password@localhost:5432/gophkeeper?sslmode=disable"
export JWT_SECRET="your-secret-key"
export GRPC_PORT="50051"
export STORAGE_ENDPOINT="localhost:9000"
export STORAGE_ACCESS_KEY="minioadmin"
export STORAGE_SECRET_KEY="minioadmin"
export STORAGE_BUCKET="gophkeeper"
export KDF_TIME=1
export KDF_MEM_KIB=64
export KDF_PAR=1
export LOG_LEVEL=0
```

## Для пользователей

### Запуск через Docker Compose (рекомендуется)
```bash
# Запуск всей инфраструктуры (PostgreSQL + MinIO + сервер)
docker-compose up -d postgres minio server

# Сервер будет доступен на порту 50051
# MinIO консоль на http://localhost:9001 (admin/minioadmin)
```

### Самостоятельный запуск
Перед запуском убедитесь, что:
1. PostgreSQL работает и база данных создана
2. MinIO работает и bucket создан
3. Применены миграции базы данных

```bash
# Сборка
go build -o gophkeeper-server ./cmd

# Запуск с переменными окружения
DATABASE_DSN="postgres://user:password@localhost:5432/gophkeeper?sslmode=disable" \
JWT_SECRET="your-jwt-secret" \
STORAGE_ENDPOINT="localhost:9000" \
STORAGE_ACCESS_KEY="minioadmin" \
STORAGE_SECRET_KEY="minioadmin" \
STORAGE_BUCKET="gophkeeper" \
./gophkeeper-server
```

### Проверка работоспособности
```bash
# Проверка gRPC сервера с помощью grpcurl
grpcurl -plaintext localhost:50051 list

# Проверка health endpoint (если реализован)
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
```

## Допущения MVP и возможности развития
- только один алгоритм шифрования
- обновление через удаление + создание
- нет фетчинга данных