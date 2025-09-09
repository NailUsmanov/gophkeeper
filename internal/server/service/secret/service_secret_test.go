package secret

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
	insertFn       func(ctx context.Context, s *Secret) error
	getByIDFn      func(ctx context.Context, ownerID, secretID string) (*Secret, error)
	listFn         func(ctx context.Context, ownerID string, limit, offset int, filter SecretListFilter) ([]*Secret, int, error)
	updateFn       func(ctx context.Context, ownerID string, s *Secret) error
	softDeleteFn   func(ctx context.Context, ownerID, secretID string, now time.Time) error
	updatedAfterFn func(ctx context.Context, ownerID string, since time.Time) ([]*Secret, error)
}

func (m repoMock) Insert(ctx context.Context, s *Secret) error {
	if m.insertFn != nil {
		return m.insertFn(ctx, s)
	}
	return nil
}

func (m repoMock) GetByID(ctx context.Context, ownerID, secretID string) (*Secret, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, ownerID, secretID)
	}
	return nil, nil
}

func (m repoMock) List(ctx context.Context, ownerID string, limit, offset int, filter SecretListFilter) ([]*Secret, int, error) {
	if m.listFn != nil {
		return m.listFn(ctx, ownerID, limit, offset, filter)
	}
	return nil, 0, nil
}

func (m repoMock) Update(ctx context.Context, ownerID string, s *Secret) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, ownerID, s)
	}
	return nil
}

func (m repoMock) SoftDelete(ctx context.Context, ownerID, secretID string, now time.Time) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, ownerID, secretID, now)
	}
	return nil
}

func (m repoMock) UpdatedAfter(ctx context.Context, ownerID string, since time.Time) ([]*Secret, error) {
	if m.updatedAfterFn != nil {
		return m.updatedAfterFn(ctx, ownerID, since)
	}
	return nil, nil
}

// Test Create
func TestServiceCreateOK(t *testing.T) {
	fixed := time.Date(2025, 9, 6, 10, 0, 0, 0, time.UTC)

	r := repoMock{
		insertFn: func(ctx context.Context, s *Secret) error {
			// Проверим, что сервис заполнил служебные поля
			require.NotEmpty(t, s.ID)
			require.False(t, s.CreatedAt.IsZero())
			require.False(t, s.UpdatedAt.IsZero())
			require.Equal(t, 1, s.Version)
			return nil
		},
	}

	svc := NewService(r)
	// Зафиксируем время в сервисе
	svc.now = fixed

	req := CreateReq{
		Type:  models.SecretType("note"),
		Title: "My note",
		Data:  map[string]any{"text": "hello"},
	}
	got, err := svc.Create(context.Background(), "u1", req)
	require.NoError(t, err)
	require.Equal(t, "u1", got.OwnerID)
	require.Equal(t, "note", string(got.Type))
	require.Equal(t, "My note", got.Title)
	require.Equal(t, fixed, got.CreatedAt)
	require.Equal(t, fixed, got.UpdatedAt)
}

func TestServiceCreateInvalidInput(t *testing.T) {
	svc := NewService(repoMock{})
	_, err := svc.Create(context.Background(), "u1", CreateReq{
		Type:  "", // no Type
		Title: "x",
		Data:  map[string]any{"a": 1},
	})
	require.True(t, models.HasCode(err, models.ErrCodeInvalidInput.Error()))

	_, err = svc.Create(context.Background(), "u1", CreateReq{
		Type:  "note",
		Title: "", // no Title
		Data:  map[string]any{"a": 1},
	})
	require.True(t, models.HasCode(err, models.ErrCodeInvalidInput.Error()))

	_, err = svc.Create(context.Background(), "u1", CreateReq{
		Type:  "note",
		Title: "x",
		Data:  nil, // no Data
	})
	require.True(t, models.HasCode(err, models.ErrCodeInvalidInput.Error()))
}

func TestServiceCreateInsertErrorMapsToInternal(t *testing.T) {
	r := repoMock{
		insertFn: func(ctx context.Context, s *Secret) error {
			return errors.New("db down")
		},
	}
	svc := NewService(r)
	_, err := svc.Create(context.Background(), "u1", CreateReq{
		Type:  "note",
		Title: "t",
		Data:  map[string]any{"x": 1},
	})
	require.True(t, models.HasCode(err, models.ErrCodeInternal.Error()))
}

// Test GetByID
func TestServiceGetByIDOK(t *testing.T) {
	now := time.Now()
	r := repoMock{
		getByIDFn: func(ctx context.Context, ownerID, secretID string) (*Secret, error) {
			return &Secret{
				ID:        secretID,
				OwnerID:   ownerID,
				Type:      models.SecretType("note"),
				Title:     "T",
				Data:      map[string]any{"k": "v"},
				Version:   3,
				CreatedAt: now.Add(-time.Hour),
				UpdatedAt: now,
			}, nil
		},
	}

	svc := NewService(r)

	got, err := svc.GetByID(context.Background(), "u1", "s1")
	require.NoError(t, err)
	require.Equal(t, "s1", got.ID)
	require.Equal(t, "u1", got.OwnerID)
	require.Equal(t, models.SecretType("note"), got.Type)
	require.Equal(t, 3, got.Version)
	require.Equal(t, "T", got.Title)
}

func TestServiceGetByID_NotFound(t *testing.T) {
	r := repoMock{
		getByIDFn: func(ctx context.Context, ownerID, secretID string) (*Secret, error) {
			return nil, models.NewNotFound(nil)
		},
	}
	svc := NewService(r)
	_, err := svc.GetByID(context.Background(), "u1", "s1")
	require.Error(t, err, models.HasCode(err, models.ErrCodeInternal.Error()))
}

// Test List
func TestServiceListOK(t *testing.T) {
	r := repoMock{
		listFn: func(ctx context.Context, ownerID string, limit, offset int, filter SecretListFilter) ([]*Secret, int, error) {
			return []*Secret{
				{ID: "s1", OwnerID: "u1", Title: "A"},
				{ID: "s2", OwnerID: "u2", Title: "B"},
			}, 2, nil
		},
	}
	svc := NewService(r)

	items, total, err := svc.List(context.Background(), "u1", 10, 0, SecretListFilter{})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, items, 2)
	require.Equal(t, "s1", items[0].ID)
	require.Equal(t, "A", items[0].Title)
	require.Equal(t, "u1", items[0].OwnerID)
}

func TestServiceListEmptyOK(t *testing.T) {
	r := repoMock{
		listFn: func(ctx context.Context, ownerID string, limit, offset int, filter SecretListFilter) ([]*Secret, int, error) {
			return []*Secret{}, 0, nil
		},
	}
	svc := NewService(r)
	items, total, err := svc.List(context.Background(), "u1", 10, 0, SecretListFilter{})
	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.Len(t, items, 0)
}

func TestServiceListInternal(t *testing.T) {
	r := repoMock{
		listFn: func(ctx context.Context, ownerID string, limit, offset int, filter SecretListFilter) ([]*Secret, int, error) {
			return nil, 0, errors.New("db error")
		},
	}
	svc := NewService(r)
	_, _, err := svc.List(context.Background(), "u1", 10, 0, SecretListFilter{})
	require.True(t, models.HasCode(err, models.ErrCodeInternal.Error()))
}

// Test Update
func TestServiceUpdateOK(t *testing.T) {
	now := time.Now()
	r := repoMock{
		getByIDFn: func(ctx context.Context, ownerID, secretID string) (*Secret, error) {
			return &Secret{
				ID:        secretID,
				OwnerID:   ownerID,
				Type:      models.SecretType("note"),
				Title:     "Old",
				Data:      map[string]any{"text": "x"},
				Version:   3,
				CreatedAt: now.Add(-2 * time.Hour),
				UpdatedAt: now.Add(-time.Hour),
			}, nil
		},
		updateFn: func(ctx context.Context, ownerID string, s *Secret) error {
			require.Equal(t, 4, s.Version)
			return nil
		},
	}

	svc := NewService(r)

	got, err := svc.Update(context.Background(), "u1", "s1", UpdateReq{
		Title:   nil,
		Data:    map[string]any{"text": "y"},
		Version: 3,
	})
	require.NoError(t, err)
	require.Equal(t, "Old", got.Title) // сохранился
	require.Equal(t, 4, got.Version)
	require.True(t, got.UpdatedAt.After(now))
}

func TestServiceUpdateOKNewTitle(t *testing.T) {
	r := repoMock{
		getByIDFn: func(ctx context.Context, ownerID, secretID string) (*Secret, error) {
			return &Secret{
				ID:      secretID,
				OwnerID: ownerID,
				Title:   "Old",
				Version: 10,
				Type:    "note",
			}, nil
		},
		updateFn: func(ctx context.Context, ownerID string, s *Secret) error {
			return nil
		},
	}
	svc := NewService(r)
	newTitle := "New"

	got, err := svc.Update(context.Background(), "u1", "s1", UpdateReq{
		Title:   &newTitle,
		Data:    map[string]any{"x": 1},
		Version: 10,
	})
	require.NoError(t, err)
	require.Equal(t, "New", got.Title)
}

func TestServiceUpdateConflict(t *testing.T) {
	r := repoMock{
		getByIDFn: func(ctx context.Context, ownerID, secretID string) (*Secret, error) {
			return &Secret{
				ID:      secretID,
				OwnerID: ownerID,
				Version: 5,
			}, nil
		},
	}
	svc := NewService(r)
	_, err := svc.Update(context.Background(), "u1", "s1", UpdateReq{
		Data:    map[string]any{"x": 1},
		Version: 4, //намеренная ошибка
	})
	require.Error(t, err, models.HasCode(err, models.ErrCodeConflict.Error()))
}

func TestServiceUpdateValidation(t *testing.T) {
	r := repoMock{
		getByIDFn: func(ctx context.Context, ownerID, secretID string) (*Secret, error) {
			return &Secret{ID: secretID, OwnerID: ownerID, Version: 1}, nil
		},
	}
	svc := NewService(r)
	_, err := svc.Update(context.Background(), "u1", "s1", UpdateReq{
		Data:    nil, // обязателен
		Version: 1,
	})
	require.Error(t, err, models.HasCode(err, models.ErrCodeValidationFail.Error()))
}
