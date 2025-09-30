package memory_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/server/storage/session/memory"
	"github.com/stretchr/testify/require"
)

func TestStore_CreateAndGet_OK(t *testing.T) {
	st := memory.NewStore()
	ctx := context.Background()

	exp := time.Now().UTC().Add(10 * time.Minute)
	require.NoError(t, st.Create(ctx, "tok1", "u1", exp))

	uid, expiresAt, revoked, err := st.Get(ctx, "tok1")
	require.NoError(t, err)
	require.Equal(t, "u1", uid)
	require.WithinDuration(t, exp, expiresAt, time.Second)
	require.False(t, revoked)
}

func TestStore_Get_NotFound(t *testing.T) {
	st := memory.NewStore()
	ctx := context.Background()

	_, _, _, err := st.Get(ctx, "nope")
	require.ErrorIs(t, err, memory.ErrNotFound)
}

func TestStore_Revoke_OK(t *testing.T) {
	st := memory.NewStore()
	ctx := context.Background()

	exp := time.Now().UTC().Add(5 * time.Minute)
	require.NoError(t, st.Create(ctx, "tokX", "userX", exp))

	require.NoError(t, st.Revoke(ctx, "tokX"))

	uid, expiresAt, revoked, err := st.Get(ctx, "tokX")
	require.NoError(t, err)
	require.Equal(t, "userX", uid)
	require.WithinDuration(t, exp, expiresAt, time.Second)
	require.True(t, revoked, "token must be marked as revoked after Revoke")
}

func TestStore_Revoke_NotFound(t *testing.T) {
	st := memory.NewStore()
	err := st.Revoke(context.Background(), "no-such-token")
	require.ErrorIs(t, err, memory.ErrNotFound)
}

func TestStore_Create_Overwrite(t *testing.T) {
	st := memory.NewStore()
	ctx := context.Background()

	exp1 := time.Now().UTC().Add(1 * time.Minute)
	require.NoError(t, st.Create(ctx, "dup", "u1", exp1))

	// перезаписываем тем же ключом
	exp2 := time.Now().UTC().Add(2 * time.Minute)
	require.NoError(t, st.Create(ctx, "dup", "u2", exp2))

	uid, expiresAt, revoked, err := st.Get(ctx, "dup")
	require.NoError(t, err)
	require.Equal(t, "u2", uid, "last Create should overwrite userID")
	require.WithinDuration(t, exp2, expiresAt, time.Second)
	require.False(t, revoked)
}

func TestStore_Create_ZeroExpiry(t *testing.T) {
	st := memory.NewStore()
	ctx := context.Background()

	var zero time.Time
	require.NoError(t, st.Create(ctx, "zero", "uZ", zero))

	uid, expiresAt, revoked, err := st.Get(ctx, "zero")
	require.NoError(t, err)
	require.Equal(t, "uZ", uid)
	require.True(t, expiresAt.IsZero(), "expiresAt should be zero when stored as zero")
	require.False(t, revoked)
}

// ВАЖНО: текущая реализация Store не скрывает просроченные токены в Get.
// Проверяем, что expiresAt действительно в прошлом, но запись возвращается.
func TestStore_ExpiredToken_ReturnsRecord(t *testing.T) {
	st := memory.NewStore()
	ctx := context.Background()

	expired := time.Now().UTC().Add(-1 * time.Minute)
	require.NoError(t, st.Create(ctx, "tok-exp", "uE", expired))

	uid, expiresAt, revoked, err := st.Get(ctx, "tok-exp")
	require.NoError(t, err)
	require.Equal(t, "uE", uid)
	require.True(t, time.Now().UTC().After(expiresAt), "expiresAt should be in the past")
	require.False(t, revoked)
}

// Простой конкурентный сценарий: параллельные Create/Get/Revoke.
// Цель — пройти через блокировки без паник/дедлоков.
func TestStore_ConcurrentAccess_NoRace(t *testing.T) {
	st := memory.NewStore()
	ctx := context.Background()

	var wg sync.WaitGroup

	// writer goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			exp := time.Now().UTC().Add(time.Duration(i) * time.Minute)
			_ = st.Create(ctx, "tok-conc", "user", exp)
		}()
	}

	// reader goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _, _ = st.Get(ctx, "tok-conc")
		}()
	}

	// revoker goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = st.Revoke(ctx, "tok-conc")
		}()
	}

	wg.Wait()

	// финальная проверка: запись существует, revoked либо true, либо false — в зависимости от гонки Revoke/Create.
	_, _, _, err := st.Get(ctx, "tok-conc")
	require.NoError(t, err)
}
