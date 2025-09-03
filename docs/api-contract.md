Этот документ описывает контракт API для сервера GophKeeper.

Обзор:
Формат: JSON (UTF-8)
Аутентификация: cookie auth_token
Версионирование API: /api/v1 (базовый префикс маршрутов)

1. Аутентификация:
    - Способ:
        через cookie auth_token
    - Где берется:
        POST /register - устанавливает cookie
        POST /login - устанавливает/обновляет cookie
    - В каждом запросе к защищенным эндпоинтам клиент должен присылать cookie auth_token.
    - Коды ошибок:
        Отсутствует/ невалидно - 401 unauthorized
    - Срок жизни cookie: один час.
    - Logout: стирает cookie(Max-Age = 0)

2. Сущности:
    2.1 User:
        id - string
        email - string
        created_at - RFC 3339

    2.2 Secret:
        id - string
        owner_id - string(User.id)
        type - password || note || card || file
        title - string
        data - object:
            - password: { login, password, url, comment } (в ответе password можно не возвращать или маскировать)
            - note: { text }
            - card: { holder, number_masked, expiry, cvc_masked } (в ответе полные number/cvc не возвращаем)
            file: { attachment_id, name?, size? } (метаданные привязки к вложению)
        tags - string[]
        version - number инкремент
        created_at - время RFC 3339
        updated_at - время RFC 3339
        deleted_at - время RFC 3339

    2.3 Attachment:
        id: string
		secret_id: string
		size: number
		content_type: string
		checksum: string
		created_at: RFC 3339

3. Эндпоинты:
    3.1 Health:
        GET /health
        - 200 OK -> {"status": "ok"}
        - 500 -> Internal server error

    3.2 Auth:
        POST /api/v1/register
        - Request: {"email":string, "password": string}
        - Response: {"user": ...} + ставят cookie auth_token
        - Ошибки: 400 invalid_input, 401 unauthorized, 409 conflict(если email уже есть)

        POST /api/v1/login
        - Request: {"email":string, "password": string}
        - Response: {"user": ...} + ставят cookie auth_token
        - Ошибки: 400 invalid_input, 401 unauthorized
        
        POST /api/v1/logout
        - Response: 204 (и очищает cookie)

    3.3 Secrets:
        POST /api/v1/secrets - создать секрет
        - Request:
            JSON {
            "type": "password",
            "title":"github",
            "data":{"login":"exampleLogin", "password":"1234", "url":"https://github.com"},
            }
        - Response: 201 {"secret": {..полная модель..}}
        - Ошибки: 400 invalid_input, 401 unauthorized, 422 validation_failed

        GET /api/v1/secrets/{id} - получить секрет
        - Response: {"secret":{ ... }}
        - Ошибки: 401 unauthorized, 404 not_found
        
        GET /api/v1/secrets - получить список всех секретов
        - Query: limit(1..100) - сколько выдать, offset - смещение(>=0)
        - Response: 201 
            {
            "items": [
                { "id":"...", "title":"...", "type":"...", "tags":["..."], "updated_at":"..." }
            ],
            "total": 123,
            "limit": 20,
            "offset": 0
            }
        - Ошибки: 400, 404 

        PUT /api/v1/secrets/{id} - обновить секрет (целый ресурс)
        - Request: { ... вся модель ... } - обязан передавать "version": N
        - Response: 200 {"secret": { ... }} - возвращает новую "version": N+1
        - Ошибки: 400, 401, 404, 409, 422

        DELETE /api/v1/secrets/{id}
        - Soft-delete (помечаем как удаленное)
        - Response: 204
        - Ошибки: 401, 404
    
    3.4 Files/Attachment
        POST /api/v1/attachments - загрузка
        - Response: 201 { "attachment": { "id":"...","secret_id":"...","size":12345,"content_type":"image/png","checksum":"..." } }
        - Ошибки: 400 invalid_input, 401, 413 (слишком большой файл), 415 (неподдерживаемый тип), 500

        GET /api/v1/attachments/{id}/download - скачивание
        - Response: 200 c Content-Type исходного файла и с самим байтовым телом.
    3.5 Синхронизация
        GET /api/v1/sync — отдать изменения после указанного момента (защищено)
        Query: since — RFC 3339 (время последней синхронизации клиента)
        200 →
            {
                "updated": [
                    { "secret": { ...полная модель... } }
                ],
                "deleted": [
                    { "id": "..." }
                ],
                "server_time": "2025-09-01T10:00:00Z"
            }


4. Валидаторы:
    - Ограничение по длине title, полей внутри data по типам.
    - Для card: форматы number/expiry + запрет на возврат cvc.
    - Для password: min/max длины, допустимые символы.
    - Допустимые символы/форматы (email/URL/карта)
    - Лимиты на размер вложений (если файлы)
    - Что считается валидным/невалидным -> ведет к 400 или к 422.

5. Единый формат ответа/ошибки
    5.1 Ошибка JSON {
        "error": "string-code",
        "message":"human readable",
        "details":{"title":"reason"}
    }
    Примеры кодов:
	•	invalid_input → 400
	•	unauthorized → 401
	•	forbidden → 403
	•	not_found → 404
	•	conflict → 409
	•	validation_failed → 422
	•	internal_error → 500

6. Версионирование и конкурентные обновления.
    Подход: optimistic locking
        Клиент читает secret с version = N
        При PUT отправляет version = N
        Если на сервере уже version = N+1 -> 409 conflict(version_mismatch)
        Сервер возвращает актуальный ресурс с новой version

