package postgres

// QueryInsertSecret запрос для Postgre на вставку в базу данных.
var QueryInsertSecret string = `
	INSERT INTO secrets
		(id, owner_id, type, title, data, version, created_at, updated_at, deleted_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, NULL)
`

// QueryGetByIDSecret для получения Секрета
var QueryGetByIDSecret string = `
	SELECT id, owner_id, type, title, data, version, created_at, updated_at
	FROM secrets
	WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL
	LIMIT 1
`

// QueryListSecret для получения нескольких секретов с пагинацией.
var QueryListSecret string = `
	SELECT id, owner_id, type, title, data, version, created_at,updated_at 
	FROM secrets
	WHERE owner_id = $1 AND deleted_at IS NULL
`

// QueryCountSecret для подсчета общего числа секретов.
var QueryCountSecret string = `
	SELECT COUNT(*)
	FROM secrets
	WHERE owner_id = $1 AND deleted_at IS NULL
`

// QueryUpdate для обновления секрета.
var QueryUpdate string = ` 
UPDATE secrets
SET title = $1,
	data = $2::jsonb,
	version = $3,
	updated_at = $4
WHERE id = $5 AND owner_id = $6 AND deleted_at IS NULL AND version = $7
`

// CreateUserQuery для создания пользователя.
var CreateUserQuery string = `
	INSERT INTO users
	(id, email, created_at, password_hash)
	VALUES ($1,$2,$3,$4)
`

// FindUserByEmailQuery для поиска пользователя по Email.
var FindUserByEmailQuery string = `
	SELECT id,email,password_hash,created_at 
	FROM users
	WHERE email = $1
	LIMIT 1
`

// FindUserByIDQuery для поиска пользователя по ID.
var FindUserByIDQuery string = `
	SELECT id, email, password_hash, created_at 
	FROM users
	WHERE id = $1	
	LIMIT 1
`

// InsertAttachmentQuery для внесения метаданных файла.
var InsertAttachmentQuery string = `
		INSERT INTO attachments_meta 
			(id, file_name, secret_id, owner_id, size, content_type, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
`

// GetByIDAttachmentQuery возвращает метаданные файла-секрета по ID.
var GetByIDAttachmentQuery string = `
		SELECT id, file_name, secret_id, owner_id, size, content_type, created_at
		FROM attachments_meta
		WHERE id = $1
		LIMIT 1
	`

// DeleteAttachmentQuery для удаления метаданных из БД.
var DeleteAttachmentQuery string = `
	DELETE FROM attachments_meta
	WHERE id = $1
	`

// ListAttachmentQuery выдает списком все метаданные конкретного секрета для определенного пользователя.
var ListAttachmentQuery string = `
	SELECT id, file_name, secret_id, owner_id, size, content_type, created_at
	FROM attachments_meta
	WHERE secret_id = $1 AND owner_id = $2
	ORDER BY created_at DESC
	LIMIT $3 OFFSET $4
`

// CountAttachmentQuery для подсчета количества метаданных пользователя.
var CountAttachmentQuery string = `
	SELECT COUNT(*)
	FROM attachments_meta
	WHERE owner_id = $1 AND secret_id = $2
	`
