package models_test

import (
	"testing"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/stretchr/testify/require"
)

func TestAppError_ConstructorsAndHelpers(t *testing.T) {
	t.Run("InvalidInput", func(t *testing.T) {
		err := models.NewInvalidInput(map[string]any{"field": "x"})
		require.Equal(t, "invalid input", err.Error())
		require.Equal(t, 400, err.Status())
		require.True(t, models.HasCode(err, models.ErrCodeInvalidInput.Error()))
	})

	t.Run("Unauthorized", func(t *testing.T) {
		err := models.NewUnauthorized(nil)
		require.Equal(t, "not authorized", err.Error())
		require.Equal(t, 401, err.Status())
		require.True(t, models.HasCode(err, models.ErrCodeUnauthorized.Error()))
	})

	t.Run("Forbidden", func(t *testing.T) {
		err := models.NewForbidden(nil)
		require.Equal(t, "forbidden", err.Error())
		require.Equal(t, 403, err.Status())
		require.True(t, models.HasCode(err, models.ErrCodeForbidden.Error()))
	})

	t.Run("NotFound", func(t *testing.T) {
		err := models.NewNotFound(nil)
		require.Equal(t, "not found", err.Error())
		require.Equal(t, 404, err.Status())
		require.True(t, models.HasCode(err, models.ErrCodeNotFound.Error()))
	})

	t.Run("Conflict", func(t *testing.T) {
		err := models.NewConflict(nil)
		require.Equal(t, "conflict", err.Error())
		require.Equal(t, 409, err.Status())
		require.True(t, models.HasCode(err, models.ErrCodeConflict.Error()))
	})

	t.Run("Validation", func(t *testing.T) {
		err := models.NewValidation(map[string]any{"v": 1})
		require.Equal(t, "validation failed", err.Error())
		require.Equal(t, 422, err.Status())
		require.True(t, models.HasCode(err, models.ErrCodeValidationFail.Error()))
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		err := models.NewAlreadyExists(nil)
		require.Equal(t, "already exists", err.Error())
		require.Equal(t, 409, err.Status())
		require.True(t, models.HasCode(err, models.ErrCodeAlreadyExists.Error()))
	})

	t.Run("BadRequest", func(t *testing.T) {
		err := models.NewBadRequest(nil)
		require.Equal(t, "bad request", err.Error())
		require.Equal(t, 400, err.Status())
		require.True(t, models.HasCode(err, models.ErrCodeBadRequest.Error()))
	})

}

func TestNewInternal_BuildsAppError(t *testing.T) {
	err := models.NewInternal(nil)
	require.Equal(t, models.ErrCodeInternal.Error(), err.Code)
	require.Equal(t, "internal server error", err.Message) // поправь текст, если у тебя другой
	require.True(t, models.HasCode(err, models.ErrCodeInternal.Error()))
}
