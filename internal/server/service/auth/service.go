// Package auth содержит интерфейс бизнес-логики AuthService и интерфейс доступа к данным UserRepository.
// Хендлеры работают только с AuthService,
// а тот внутри вызывает UserRepository для работы с БД.
package auth

import (
	"context"

	"github.com/NailUsmanov/gophkeeper/internal/models"
)

// AuthService — бизнес-логика регистрации/логина.
// Cервис возвращает и user, и authToken — хендлер поставит cookie.
type AuthService interface {
	Register(ctx context.Context, email, password string) (*models.User, string, error) // создать пользователя, захешировать пароль, выдать токен
	Login(ctx context.Context, email, password string) (*models.User, string, error)    // проверить пароль, выдать токен
	Logout(ctx context.Context, authToken string) error
}

// UserRepository — низкоуровневый доступ к данным пользователя (БД).
// Здесь НЕТ бизнес-логики — только CRUD/поиск.
type UserRepository interface {
	Create(ctx context.Context, u *models.User) error
	FindUserByEmail(ctx context.Context, email string) (*models.User, error)
	FindUserByID(ctx context.Context, userID string) (*models.User, error)
}
