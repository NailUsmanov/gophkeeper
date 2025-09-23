# GophKeeper CLI

CLI-клиент для работы с системой хранения секретов **GophKeeper**.

## Возможности
- Регистрация и вход пользователей (`register`, `login`, `logout`)
- Управление секретами (`create`, `list`, `update`, `get`)
- Работа с вложенными файлами (`attachment upload`, `attachment list`, `attachment download`)
- Просмотр версии и даты сборки (`version`)

## Установка и сборка

### Локальная сборка
```bash
go build -o gk \
  -ldflags "-X 'main.buildVersion=1.0.0' \
            -X 'main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)' \
            -X 'main.buildCommit=$(git rev-parse --short HEAD)'" \
  ./cmd/client

  # Linux
GOOS=linux GOARCH=amd64 go build -o gk ./cmd/client

# Windows
GOOS=windows GOARCH=amd64 go build -o gk.exe ./cmd/client

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o gk ./cmd/client

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o gk ./cmd/client

# показать версию клиента
gk version

# регистрация нового пользователя
gk register --email user@example.com --password secret

# вход в систему
gk login --email user@example.com --password secret

# создание заметки
gk secret create --type note --title "My note" --data '{"note":"hi"}'

# список секретов
gk secret list

# загрузка вложения
gk attachment upload --secret-id <uuid> --file ./demo.txt

# список вложений
gk attachment list --secret-id <uuid>

# скачивание вложения
gk attachment download <attachment-id> --dest ./out.txt

# выход из системы
gk logout