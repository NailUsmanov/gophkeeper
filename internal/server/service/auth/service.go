// Package auth содержит бизнес-логику аутентификации и регистрации пользователей.
//
// Хендлеры HTTP-слоя работают только с AuthService.
// AuthService внутри обращается к UserRepository для работы с БД.
// PasswordHasher(хэширование паролей) и TokenManager (выдача/отзыв токенов).
package auth

import (
	"context"
	"strings"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/google/uuid"
)

// PasswordHasher описывает интерфейс для работы с паролями.
// Используется AuthService для хэширования пароля при регистрации и проверки пароля при логине.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error // nil == ok
}

// TokenManager описывает интерфейс для работы с токенами.
// Используется для выдачи токена пользователю и отзыва токена при выходе.

type TokenManager interface {
	Issue(ctx context.Context, userID string) (string, error)
	Revoke(ctx context.Context, token string) error
}

// UserRepository описывает минимальные операции с пользователями на уровне БД.
// Здесь НЕТ бизнес-логики — только CRUD/поиск.
type UserRepository interface {
	Create(ctx context.Context, u *models.User) error
	FindUserByEmail(ctx context.Context, email string) (*models.User, error)
	FindUserByID(ctx context.Context, userID string) (*models.User, error)
}

// AuthService реализует бизнес-логику регистрации, входа и выхода пользователя.
// Установка cookie с токеном выполняется уже в HTTP-хендлере.
type AuthService struct {
	repo   UserRepository
	hasher PasswordHasher
	tokens TokenManager
}

// NewAuthService создаёт новый экземпляр AuthService с заданными зависимостями.
func NewAuthService(repo UserRepository, hasher PasswordHasher, tokens TokenManager) *AuthService {
	return &AuthService{
		repo:   repo,
		hasher: hasher,
		tokens: tokens,
	}
}

// Register регистрирует нового пользователя.
// Алгоритм:
//  1. Проверяем, что пользователя с таким email ещё нет.
//  2. Хэшируем пароль.
//  3. Создаём запись о пользователе в БД.
//  4. Выписываем токен для пользователя.
//
// Возвращает созданного пользователя и токен.
func (a *AuthService) Register(ctx context.Context, email, password string) (*models.User, string, error) {
	// 1) Проверяем, нет ли уже такого email.
	if existing, err := a.repo.FindUserByEmail(ctx, email); err == nil && existing != nil {
		// пользователь найден → это не успех регистрации
		return nil, "", models.NewAlreadyExists(nil)
	} else if err != nil && !models.HasCode(err, models.ErrCodeNotFound.Error()) {
		// иная ошибка репозитория
		return nil, "", models.NewInternal(map[string]any{"op": "FindUserByEmail"})
	}

	// 2) Хэшируем пароль.
	hash, err := a.hasher.Hash(password)
	if err != nil {
		return nil, "", models.NewInternal(map[string]any{"op": "Hash"})
	}

	// 3) Создаем пользователя.
	newUser := models.User{
		ID:           generateID(),
		Email:        email,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}
	if err := a.repo.Create(ctx, &newUser); err != nil {
		return nil, "", models.NewInternal(map[string]any{"op": "Create"})
	}

	// 4) Выписываем токен
	token, err := a.tokens.Issue(ctx, newUser.ID)
	if err != nil {
		return nil, "", models.NewInternal(map[string]any{"op": "IssueToken"})
	}
	return &newUser, token, nil
}

// Login выполняет вход пользователя (по email и паролю).
// Алгоритм:
//  1. Найти пользователя по email.
//  2. Проверить введенный пароль.
//  3. Выдать новый токен.
func (a *AuthService) Login(ctx context.Context, email, password string) (*models.User, string, error) {
	// 1. Находим пользователя по email.
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := a.repo.FindUserByEmail(ctx, email)
	if err != nil {
		if models.HasCode(err, models.ErrCodeNotFound.Error()) {
			return nil, "", models.NewUnauthorized(nil)
		}
		// Иная ошибка репозитория — внутренняя.
		return nil, "", models.NewInternal(map[string]any{"op": "FindUserByEmail"})
	}
	if user == nil {
		return nil, "", models.NewUnauthorized(nil)
	}

	// 2. Проверяем хэш пароля.
	if err := a.hasher.Compare(user.PasswordHash, password); err != nil {
		return nil, "", models.NewUnauthorized(nil)
	}

	// 3. Выдаем новый токен.
	tok, err := a.tokens.Issue(ctx, user.ID)
	if err != nil {
		return nil, "", models.NewInternal(map[string]any{"op": "IssueToken"})
	}

	return user, tok, nil
}

// Logout выполняет выход пользователя.
// Должен отозвать (Revoke) указанный токен.
func (a *AuthService) Logout(ctx context.Context, authToken string) error {
	if err := a.tokens.Revoke(ctx, authToken); err != nil {
		return models.NewInternal(map[string]any{"op": "RevokeToken"})
	}
	return nil
}

func generateID() string {
	return uuid.New().String()
}
