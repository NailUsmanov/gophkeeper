package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/stretchr/testify/require"
)

// Мок репозитория (функциональные поля под каждый метод)

type repoMock struct {
	createFn          func(ctx context.Context, u *models.User) error
	findByUserEmailFn func(ctx context.Context, email string) (*models.User, error)
	findByUserIDFn    func(ctx context.Context, userID string) (*models.User, error)
}

func (m *repoMock) Create(ctx context.Context, u *models.User) error {
	if m.createFn != nil {
		return m.createFn(ctx, u)
	}
	return nil
}

func (m *repoMock) FindUserByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.findByUserEmailFn != nil {
		return m.findByUserEmailFn(ctx, email)
	}
	return nil, nil
}

func (m *repoMock) FindUserByID(ctx context.Context, userID string) (*models.User, error) {
	if m.findByUserIDFn != nil {
		return m.findByUserIDFn(ctx, userID)
	}
	return nil, nil
}

// hasherMock — мок PasswordHasher.
type hasherMock struct {
	hashFn    func(password string) (string, error)
	compareFn func(hash, password string) error
}

func (m *hasherMock) Hash(password string) (string, error) {
	if m.hashFn != nil {
		return m.hashFn(password)
	}
	return "HASHED:" + password, nil
}
func (m *hasherMock) Compare(hash, password string) error {
	if m.compareFn != nil {
		return m.compareFn(hash, password)
	}
	// по умолчанию — успех
	return nil
}

// tokensMock — мок TokenManager.
type tokensMock struct {
	issueFn  func(ctx context.Context, userID string) (string, error)
	revokeFn func(ctx context.Context, token string) error
}

func (m *tokensMock) Issue(ctx context.Context, userID string) (string, error) {
	if m.issueFn != nil {
		return m.issueFn(ctx, userID)
	}
	return "tok-123", nil
}
func (m *tokensMock) Revoke(ctx context.Context, token string) error {
	if m.revokeFn != nil {
		return m.revokeFn(ctx, token)
	}
	return nil
}

func userFixture(email string, passwordHash string) *models.User {
	return &models.User{
		ID:           "u-1",
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().UTC(),
	}
}

func asAppError(t *testing.T, err error) *models.AppError {
	t.Helper()
	var app *models.AppError
	require.Error(t, err)
	require.True(t, errors.As(err, &app), "err must be *models.AppError (or wrapped)")
	return app
}

// Test Create
func TestRegister(t *testing.T) {

	type args struct {
		email string
		pass  string
	}

	tests := []struct {
		name   string
		args   args
		repo   *repoMock
		hasher *hasherMock
		tokens *tokensMock

		wantToken   string
		wantErr     bool
		assertExtra func(t *testing.T, u *models.User, repo *repoMock, hasher *hasherMock, tokens *tokensMock, err error)
	}{
		{
			name: "OK",
			args: args{"alice@example.com", "secret"},
			repo: &repoMock{
				findByUserEmailFn: func(ctx context.Context, email string) (*models.User, error) {
					return nil, models.ErrCodeNotFound
				},
				createFn: func(ctx context.Context, u *models.User) error {
					require.NotEmpty(t, u.ID)
					require.NotZero(t, u.CreatedAt)
					require.Equal(t, "alice@example.com", u.Email)
					require.NotEmpty(t, u.PasswordHash)
					return nil
				},
			},
			hasher: &hasherMock{
				hashFn: func(pw string) (string, error) { return "H:" + pw, nil },
			},
			tokens:    &tokensMock{issueFn: func(ctx context.Context, userID string) (string, error) { return "tok-OK", nil }},
			wantToken: "tok-OK",
			assertExtra: func(t *testing.T, u *models.User, repo *repoMock, hasher *hasherMock, tokens *tokensMock, err error) {
				require.NoError(t, err)
				require.NotNil(t, u)
			},
		},
		{
			name: "AlreadyExists",
			args: args{"alice@example.com", "secret"},
			repo: &repoMock{
				findByUserEmailFn: func(ctx context.Context, email string) (*models.User, error) {
					return userFixture(email, "HASH"), nil
				},
			},
			hasher:  &hasherMock{},
			tokens:  &tokensMock{},
			wantErr: true,
			assertExtra: func(t *testing.T, u *models.User, repo *repoMock, hasher *hasherMock, tokens *tokensMock, err error) {
				app := asAppError(t, err)
				_ = app // при желании проверь код/статус
				require.Nil(t, u)
			},
		},
		{
			name: "HasherError",
			args: args{"alice@example.com", "secret"},
			repo: &repoMock{
				findByUserEmailFn: func(ctx context.Context, email string) (*models.User, error) {
					return nil, models.ErrCodeNotFound
				},
			},
			hasher: &hasherMock{
				hashFn: func(pw string) (string, error) { return "", errors.New("hash fail") },
			},
			tokens:  &tokensMock{},
			wantErr: true,
			assertExtra: func(t *testing.T, u *models.User, repo *repoMock, hasher *hasherMock, tokens *tokensMock, err error) {
				asAppError(t, err)
				require.Nil(t, u)
			},
		},
		{
			name: "RepoCreateError",
			args: args{"alice@example.com", "secret"},
			repo: &repoMock{
				findByUserEmailFn: func(ctx context.Context, email string) (*models.User, error) {
					return nil, models.ErrCodeNotFound
				},
				createFn: func(ctx context.Context, u *models.User) error { return errors.New("db down") },
			},
			hasher:  &hasherMock{},
			tokens:  &tokensMock{},
			wantErr: true,
			assertExtra: func(t *testing.T, u *models.User, repo *repoMock, hasher *hasherMock, tokens *tokensMock, err error) {
				asAppError(t, err)
				require.Nil(t, u)
			},
		},
		{
			name: "TokenIssueError",
			args: args{"alice@example.com", "secret"},
			repo: &repoMock{
				findByUserEmailFn: func(ctx context.Context, email string) (*models.User, error) {
					return nil, models.ErrCodeNotFound
				},
			},
			hasher:  &hasherMock{},
			tokens:  &tokensMock{issueFn: func(ctx context.Context, userID string) (string, error) { return "", errors.New("issue fail") }},
			wantErr: true,
			assertExtra: func(t *testing.T, u *models.User, repo *repoMock, hasher *hasherMock, tokens *tokensMock, err error) {
				asAppError(t, err)
				require.Nil(t, u)
			},
		},
	}

	for _, tc := range tests {
		tc := tc // capture
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := NewAuthService(tc.repo, tc.hasher, tc.tokens)
			u, tok, err := svc.Register(context.Background(), tc.args.email, tc.args.pass)
			if tc.wantErr {
				require.Error(t, err)
				require.Empty(t, tok)
			} else {
				require.NoError(t, err)
				require.NotNil(t, u)
				require.Equal(t, tc.wantToken, tok)
			}
			if tc.assertExtra != nil {
				tc.assertExtra(t, u, tc.repo, tc.hasher, tc.tokens, err)
			}
		})
	}
}

// TestLogin
func TestLogin_Table(t *testing.T) {
	type args struct {
		email string
		pass  string
	}

	tests := []struct {
		name        string
		args        args
		repo        *repoMock
		hasher      *hasherMock
		tokens      *tokensMock
		wantToken   string
		wantErr     bool
		assertExtra func(t *testing.T, u *models.User, repo *repoMock, hasher *hasherMock, tokens *tokensMock, err error)
	}{
		{
			name: "OK",
			args: args{"  Alice@example.com  ", "secret"},
			repo: &repoMock{
				findByUserEmailFn: func(ctx context.Context, email string) (*models.User, error) {
					// твой код нормализует email → сравнивать можно с нижним регистром в сервисе.
					require.Equal(t, "alice@example.com", email)
					return userFixture(email, "HASH"), nil
				},
			},
			hasher: &hasherMock{compareFn: func(hash, pw string) error {
				require.Equal(t, "HASH", hash)
				require.Equal(t, "secret", pw)
				return nil
			}},
			tokens:    &tokensMock{issueFn: func(ctx context.Context, userID string) (string, error) { return "tok-login", nil }},
			wantToken: "tok-login",
			assertExtra: func(t *testing.T, u *models.User, repo *repoMock, hasher *hasherMock, tokens *tokensMock, err error) {
				require.NoError(t, err)
				require.NotNil(t, u)

			},
		},
		{
			name: "NotFound -> Unauthorized",
			args: args{"nope@example.com", "pw"},
			repo: &repoMock{
				findByUserEmailFn: func(ctx context.Context, email string) (*models.User, error) {
					return nil, models.ErrCodeNotFound
				},
			},
			hasher:  &hasherMock{},
			tokens:  &tokensMock{},
			wantErr: true,
			assertExtra: func(t *testing.T, u *models.User, repo *repoMock, hasher *hasherMock, tokens *tokensMock, err error) {
				_ = asAppError(t, err) // обычно 401
				require.Nil(t, u)

			},
		},
		{
			name: "WrongPassword -> Unauthorized",
			args: args{"alice@example.com", "bad"},
			repo: &repoMock{
				findByUserEmailFn: func(ctx context.Context, email string) (*models.User, error) {
					return userFixture(email, "HASH"), nil
				},
			},
			hasher:  &hasherMock{compareFn: func(hash, pw string) error { return errors.New("mismatch") }},
			tokens:  &tokensMock{},
			wantErr: true,
			assertExtra: func(t *testing.T, u *models.User, repo *repoMock, hasher *hasherMock, tokens *tokensMock, err error) {
				_ = asAppError(t, err) // обычно 401
				require.Nil(t, u)

			},
		},
		{
			name: "Repo internal error -> Internal",
			args: args{"alice@example.com", "pw"},
			repo: &repoMock{
				findByUserEmailFn: func(ctx context.Context, email string) (*models.User, error) {
					return nil, errors.New("db error")
				},
			},
			hasher:  &hasherMock{},
			tokens:  &tokensMock{},
			wantErr: true,
			assertExtra: func(t *testing.T, u *models.User, repo *repoMock, hasher *hasherMock, tokens *tokensMock, err error) {
				_ = asAppError(t, err) // обычно 500
				require.Nil(t, u)

			},
		},
		{
			name: "Token issue error -> Internal",
			args: args{"alice@example.com", "pw"},
			repo: &repoMock{
				findByUserEmailFn: func(ctx context.Context, email string) (*models.User, error) {
					return userFixture(email, "HASH"), nil
				},
			},
			hasher:  &hasherMock{compareFn: func(hash, pw string) error { return nil }},
			tokens:  &tokensMock{issueFn: func(ctx context.Context, userID string) (string, error) { return "", errors.New("issue fail") }},
			wantErr: true,
			assertExtra: func(t *testing.T, u *models.User, repo *repoMock, hasher *hasherMock, tokens *tokensMock, err error) {
				_ = asAppError(t, err) // обычно 500
				require.Nil(t, u)

			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := NewAuthService(tc.repo, tc.hasher, tc.tokens)
			u, tok, err := svc.Login(context.Background(), tc.args.email, tc.args.pass)

			if tc.wantErr {
				require.Error(t, err)
				require.Empty(t, tok)
			} else {
				require.NoError(t, err)
				require.NotNil(t, u)
				require.Equal(t, tc.wantToken, tok)
			}
			if tc.assertExtra != nil {
				tc.assertExtra(t, u, tc.repo, tc.hasher, tc.tokens, err)
			}
		})
	}
}

func TestLogout_Table(t *testing.T) {
	tests := []struct {
		name        string
		tokenArg    string
		tokens      *tokensMock
		wantErr     bool
		assertExtra func(t *testing.T, tokens *tokensMock, err error)
	}{
		{
			name:     "OK",
			tokenArg: "tok-1",
			tokens: &tokensMock{
				revokeFn: func(ctx context.Context, token string) error {
					require.Equal(t, "tok-1", token)
					return nil
				},
			},
			assertExtra: func(t *testing.T, tokens *tokensMock, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:     "Revoke error -> Internal",
			tokenArg: "tok-2",
			tokens: &tokensMock{
				revokeFn: func(ctx context.Context, token string) error {
					return errors.New("store down")
				},
			},
			wantErr: true,
			assertExtra: func(t *testing.T, tokens *tokensMock, err error) {
				_ = asAppError(t, err) // обычно 500
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := NewAuthService(&repoMock{}, &hasherMock{}, tc.tokens)
			err := svc.Logout(context.Background(), tc.tokenArg)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tc.assertExtra != nil {
				tc.assertExtra(t, tc.tokens, err)
			}
		})
	}
}
