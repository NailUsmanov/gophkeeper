package password_test

import (
	"testing"

	"github.com/NailUsmanov/gophkeeper/internal/security/password"
	"github.com/stretchr/testify/require"
)

// internal/security/password/bcrypt_hasher_test.go
func TestBcryptHasher(t *testing.T) {
	h := password.NewBcryptHasher(10)
	hash, err := h.Hash("secret")
	require.NoError(t, err)
	require.NotEmpty(t, hash)

	require.NoError(t, h.Compare(hash, "secret"))
	require.Error(t, h.Compare(hash, "wrong"))
}
