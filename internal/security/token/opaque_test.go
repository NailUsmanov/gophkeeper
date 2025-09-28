package token

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/server/storage/session/memory"
	"github.com/stretchr/testify/require"
)

// простая фейковая реализация SessionStore с хуками/счётчиками
type fakeStore struct {
	createFn     func(ctx context.Context, tok, uid string, exp time.Time) error
	getFn        func(ctx context.Context, tok string) (string, time.Time, bool, error)
	revokeFn     func(ctx context.Context, tok string) error
	createCalls  int
	lastTok      string
	lastUID      string
	lastExpireAt time.Time
}

func (f *fakeStore) Create(ctx context.Context, tok, uid string, exp time.Time) error {
	f.createCalls++
	f.lastTok, f.lastUID, f.lastExpireAt = tok, uid, exp
	if f.createFn != nil {
		return f.createFn(ctx, tok, uid, exp)
	}
	return nil
}
func (f *fakeStore) Get(ctx context.Context, tok string) (string, time.Time, bool, error) {
	if f.getFn != nil {
		return f.getFn(ctx, tok)
	}
	return "", time.Time{}, false, errors.New("not implemented")
}
func (f *fakeStore) Revoke(ctx context.Context, tok string) error {
	if f.revokeFn != nil {
		return f.revokeFn(ctx, tok)
	}
	return nil
}

func TestIssue_OK(t *testing.T) {
	// используем боевое in-memory хранилище — полностью валидный путь
	st := memory.NewStore()
	m := NewOpaqueManager(st, 30*time.Minute)

	ctx := context.Background()
	tok, err := m.Issue(ctx, "u1")
	require.NoError(t, err)
	require.NotEmpty(t, tok)
	// токен — hex от 32 байт => длина 64 символа
	require.Len(t, tok, 64)
}

func TestIssue_CreateError(t *testing.T) {
	st := &fakeStore{
		createFn: func(ctx context.Context, tok, uid string, exp time.Time) error {
			return errors.New("db down")
		},
	}
	m := NewOpaqueManager(st, time.Hour)

	ctx := context.Background()
	tok, err := m.Issue(ctx, "u1")
	require.Error(t, err)
	require.Equal(t, "", tok)
	// генерация прошла, Create вызывался
	require.Equal(t, 1, st.createCalls)
	require.NotEmpty(t, st.lastTok)
	require.Equal(t, "u1", st.lastUID)
	require.WithinDuration(t, time.Now().UTC().Add(time.Hour), st.lastExpireAt, 2*time.Second)
}

func TestIssue_RandomTokenError(t *testing.T) {
	// подменяем генератор на время теста
	orig := genToken
	genToken = func(n int) (string, error) {
		return "", errors.New("rng fail")
	}
	defer func() { genToken = orig }()

	st := &fakeStore{}
	m := NewOpaqueManager(st, time.Minute)

	ctx := context.Background()
	tok, err := m.Issue(ctx, "u1")
	require.Error(t, err)
	require.Equal(t, "", tok)
	// из-за ошибки генерации Create не должен был вызываться
	require.Equal(t, 0, st.createCalls)
}

func TestValidate_OK(t *testing.T) {
	ctx := context.Background()
	st := memory.NewStore()
	m := NewOpaqueManager(st, time.Minute)

	tok, err := m.Issue(ctx, "u1")
	require.NoError(t, err)

	uid, err := m.Validate(ctx, tok)
	require.NoError(t, err)
	require.Equal(t, "u1", uid)
}
func TestValidate_StoreError(t *testing.T) {
	ctx := context.Background()
	st := &fakeStore{
		getFn: func(ctx context.Context, tok string) (string, time.Time, bool, error) {
			return "", time.Time{}, false, errors.New("store get fail")
		},
	}
	m := NewOpaqueManager(st, time.Minute)

	_, err := m.Validate(ctx, "any")
	require.Error(t, err)
}

func TestValidate_Revoked(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	st := &fakeStore{
		getFn: func(ctx context.Context, tok string) (string, time.Time, bool, error) {
			return "u1", now.Add(time.Minute), true, nil // revoked=true
		},
	}
	m := NewOpaqueManager(st, time.Minute)

	_, err := m.Validate(ctx, "tok")
	require.Error(t, err) // "token invalid"
}

func TestValidate_Expired(t *testing.T) {
	ctx := context.Background()
	st := &fakeStore{
		getFn: func(ctx context.Context, tok string) (string, time.Time, bool, error) {
			return "u1", time.Now().UTC().Add(-time.Second), false, nil // уже истёк
		},
	}
	m := NewOpaqueManager(st, time.Minute)

	_, err := m.Validate(ctx, "tok")
	require.Error(t, err)
}

func TestRevoke_CallsStore(t *testing.T) {
	ctx := context.Background()
	called := 0
	last := ""
	st := &fakeStore{
		revokeFn: func(ctx context.Context, tok string) error {
			called++
			last = tok
			return nil
		},
	}
	m := NewOpaqueManager(st, time.Minute)

	require.NoError(t, m.Revoke(ctx, "abc"))
	require.Equal(t, 1, called)
	require.Equal(t, "abc", last)
}

func TestRandomToken_LengthAndHex(t *testing.T) {
	s, err := randomToken(4) // 4 байта -> 8 hex-символов
	require.NoError(t, err)
	require.Len(t, s, 8)
	_, err = hex.DecodeString(s) // валидный hex
	require.NoError(t, err)
}
