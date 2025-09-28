package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/NailUsmanov/gophkeeper/internal/server/middlewares"
	"github.com/NailUsmanov/gophkeeper/internal/server/service/secret"
	thttp "github.com/NailUsmanov/gophkeeper/internal/server/transport/http"
	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// тихий логгер для тестов
func testLogger(t *testing.T) *zap.SugaredLogger {
	l := zap.NewNop()
	t.Cleanup(func() { _ = l.Sync() })
	return l.Sugar()
}

/* ------------------------- NewCreateSecret ------------------------- */

func TestNewCreate_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockCreateSecretService(ctrl)
	mockSvc.EXPECT().
		Create(gomock.Any(), "u1",
			gomock.AssignableToTypeOf(secret.CreateReq{}),
		).DoAndReturn(func(ctx context.Context, ownerID string, req secret.CreateReq) (*secret.Secret, error) {
		require.Equal(t, models.SecretType("note"), req.Type)
		require.Equal(t, "hello", req.Title)
		require.Equal(t, map[string]any{"x": float64(1)}, req.Data) // числа после json → float64
		return &secret.Secret{
			ID:        "s1",
			OwnerID:   "u1",
			Type:      "note",
			Title:     "hello",
			Data:      map[string]any{"x": 1},
			Version:   1,
			CreatedAt: time.Unix(10, 0).UTC(),
			UpdatedAt: time.Unix(20, 0).UTC(),
		}, nil
	})

	h := NewCreateSecret(mockSvc, testLogger(t))
	body := `{"type":"note","title":"hello","data":{"x":1}}`

	r := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusCreated, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var got thttp.SecretResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "s1", got.ID)
	require.Equal(t, "note", got.Type)
	require.Equal(t, "hello", got.Title)
	require.Equal(t, 1, got.Version)
}

func TestNewCreate_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewCreateSecret(NewMockCreateSecretService(ctrl), testLogger(t))

	r := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader(`{"type":"note","title":"t","data":{"a":1}}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestNewCreate_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewCreateSecret(NewMockCreateSecretService(ctrl), testLogger(t))

	r := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader(`{`))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	// В твоей реализации тут WriteError(models.NewInvalidInput(...)) => 400
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNewCreate_MissingFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewCreateSecret(NewMockCreateSecretService(ctrl), testLogger(t))

	r := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader(`{"type":"note","title":"","data":{}}`))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	// тоже NewInvalidInput => 400
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNewCreate_ServiceConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockCreateSecretService(ctrl)
	mockSvc.EXPECT().
		Create(gomock.Any(), "u1", gomock.AssignableToTypeOf(secret.CreateReq{})).
		Return(nil, models.NewConflict(nil))

	h := NewCreateSecret(mockSvc, testLogger(t))

	r := httptest.NewRequest(http.MethodPost, "/api/v1/secrets",
		strings.NewReader(`{"type":"note","title":"x","data":{"a":1}}`))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusConflict, w.Code)
}

func TestNewCreate_ServiceValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockCreateSecretService(ctrl)
	mockSvc.EXPECT().
		Create(gomock.Any(), "u1", gomock.AssignableToTypeOf(secret.CreateReq{})).
		Return(nil, models.NewValidation(nil))

	h := NewCreateSecret(mockSvc, testLogger(t))

	r := httptest.NewRequest(http.MethodPost, "/api/v1/secrets",
		strings.NewReader(`{"type":"note","title":"x","data":{"a":1}}`))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

/* ------------------------------ NewList ------------------------------ */

func TestNewList_OK_Defaults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockGetSecretService(ctrl)
	mockSvc.EXPECT().
		List(gomock.Any(), "u1", 20, 0, secret.SecretListFilter{}).
		Return([]*secret.Secret{
			{
				ID:        "s1",
				OwnerID:   "u1",
				Type:      "note",
				Title:     "A",
				Data:      map[string]any{"k": "v"},
				Version:   1,
				CreatedAt: time.Unix(10, 0).UTC(),
				UpdatedAt: time.Unix(20, 0).UTC(),
			},
		}, 1, nil)

	h := NewList(mockSvc, testLogger(t))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets", nil)
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	var got thttp.ListSecretResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, 20, got.Limit)
	require.Equal(t, 0, got.Offset)
	require.Equal(t, 1, got.Total)
	require.Len(t, got.Items, 1)
}

func TestNewList_OK_WithTypeAndUpdatedAfter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ua := time.Now().UTC().Add(-time.Hour)
	// проверим что фильтр реально проброшен
	mockSvc := NewMockGetSecretService(ctrl)
	mockSvc.EXPECT().
		List(gomock.Any(), "u1", 10, 5, gomock.Any()).
		DoAndReturn(func(ctx context.Context, ownerID string, limit, offset int, f secret.SecretListFilter) ([]*secret.Secret, int, error) {
			require.NotNil(t, f.Type)
			require.Equal(t, models.SecretType("note"), *f.Type)
			require.NotNil(t, f.UpdatedAfter)
			// допускаем разницу в миллисекундах — просто проверим, что значение близко
			require.WithinDuration(t, ua, *f.UpdatedAfter, time.Second*2)
			return []*secret.Secret{}, 0, nil
		})

	h := NewList(mockSvc, testLogger(t))
	url := "/api/v1/secrets?limit=10&offset=5&type=note&updated_after=" + ua.Format(time.RFC3339)
	r := httptest.NewRequest(http.MethodGet, url, nil)
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestNewList_InvalidUpdatedAfter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewList(NewMockGetSecretService(ctrl), testLogger(t))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets?updated_after=bad", nil)
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNewList_LimitTooBig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewList(NewMockGetSecretService(ctrl), testLogger(t))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets?limit=101", nil)
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

/* ----------------------------- NewGetByID ---------------------------- */

func TestGetByID_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockGetSecretService(ctrl)
	mockSvc.EXPECT().
		GetByID(gomock.Any(), "u1", "s1").
		Return(&secret.Secret{
			ID:        "s1",
			OwnerID:   "u1",
			Type:      "note",
			Title:     "T",
			Data:      map[string]any{"x": 1},
			Version:   2,
			CreatedAt: time.Unix(10, 0).UTC(),
			UpdatedAt: time.Unix(20, 0).UTC(),
		}, nil)

	h := NewGetByID(mockSvc, testLogger(t))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/s1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	var got thttp.SecretResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "s1", got.ID)
	require.Equal(t, "note", got.Type)
	require.Equal(t, 2, got.Version)
}

func TestGetByID_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewGetByID(NewMockGetSecretService(ctrl), testLogger(t))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/s1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetByID_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockGetSecretService(ctrl)
	mockSvc.EXPECT().
		GetByID(gomock.Any(), "u1", "missing").
		Return(nil, models.NewNotFound(nil))

	h := NewGetByID(mockSvc, testLogger(t))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/missing", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "missing")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetByID_Internal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockGetSecretService(ctrl)
	mockSvc.EXPECT().
		GetByID(gomock.Any(), "u1", "s1").
		Return(nil, models.NewInternal(nil))

	h := NewGetByID(mockSvc, testLogger(t))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/s1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

/* ------------------------------- NewUpdate ------------------------------- */

func TestNewUpdate_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockUpdateService(ctrl)
	mockSvc.EXPECT().
		Update(gomock.Any(), "u1", "s1", gomock.AssignableToTypeOf(secret.UpdateReq{})).
		DoAndReturn(func(ctx context.Context, ownerID, secretID string, req secret.UpdateReq) (*secret.Secret, error) {
			require.Equal(t, 3, req.Version)
			require.Nil(t, req.Title)
			require.Equal(t, map[string]any{"x": float64(1)}, req.Data)
			return &secret.Secret{
				ID:        "s1",
				OwnerID:   "u1",
				Type:      "note",
				Title:     "T",
				Data:      map[string]any{"x": 1},
				Version:   4,
				CreatedAt: time.Unix(10, 0).UTC(),
				UpdatedAt: time.Unix(20, 0).UTC(),
			}, nil
		})

	h := NewUpdate(mockSvc, testLogger(t))
	r := httptest.NewRequest(http.MethodPut, "/api/v1/secrets/s1", strings.NewReader(`{"data":{"x":1},"version":3}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	var got thttp.SecretResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "s1", got.ID)
	require.Equal(t, 4, got.Version)
}

func TestNewUpdate_ServiceInternalDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockUpdateService(ctrl)
	mockSvc.EXPECT().
		Update(gomock.Any(), "u1", "s1", gomock.AssignableToTypeOf(secret.UpdateReq{})).
		Return(nil, errors.New("boom"))

	h := NewUpdate(mockSvc, testLogger(t))
	r := httptest.NewRequest(http.MethodPut, "/api/v1/secrets/s1", strings.NewReader(`{"data":{"x":1},"version":1}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestNewUpdate_ServiceConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockUpdateService(ctrl)
	svc.EXPECT().
		Update(gomock.Any(), "u1", "s1", gomock.AssignableToTypeOf(secret.UpdateReq{})).
		Return(nil, models.NewConflict(nil))

	h := NewUpdate(svc, testLogger(t))

	r := httptest.NewRequest(http.MethodPut, "/api/v1/secrets/s1", strings.NewReader(`{"data":{"x":1},"version":3}`))
	rc := chi.NewRouteContext()
	rc.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusConflict, w.Code)
}

func TestNewUpdate_ServiceNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockUpdateService(ctrl)
	svc.EXPECT().
		Update(gomock.Any(), "u1", "s1", gomock.AssignableToTypeOf(secret.UpdateReq{})).
		Return(nil, models.NewNotFound(nil))

	h := NewUpdate(svc, testLogger(t))

	r := httptest.NewRequest(http.MethodPut, "/api/v1/secrets/s1", strings.NewReader(`{"data":{"x":1},"version":1}`))
	rc := chi.NewRouteContext()
	rc.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestNewUpdate_ServiceUnauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockUpdateService(ctrl)
	svc.EXPECT().
		Update(gomock.Any(), "u1", "s1", gomock.AssignableToTypeOf(secret.UpdateReq{})).
		Return(nil, models.NewUnauthorized(nil))

	h := NewUpdate(svc, testLogger(t))

	r := httptest.NewRequest(http.MethodPut, "/api/v1/secrets/s1", strings.NewReader(`{"data":{"x":1},"version":2}`))
	rc := chi.NewRouteContext()
	rc.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}
func TestNewUpdate_ServiceValidationOrInvalidInput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockUpdateService(ctrl)
	svc.EXPECT().
		Update(gomock.Any(), "u1", "s1", gomock.AssignableToTypeOf(secret.UpdateReq{})).
		// покрываем обе ветки через InvalidInput (в коде: validation || invalid_input -> 422)
		Return(nil, models.NewInvalidInput(nil))

	h := NewUpdate(svc, testLogger(t))

	r := httptest.NewRequest(http.MethodPut, "/api/v1/secrets/s1", strings.NewReader(`{"data":{"x":1},"version":2}`))
	rc := chi.NewRouteContext()
	rc.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestNewUpdate_OK_WithTitleTrimmed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := NewMockUpdateService(ctrl)
	svc.EXPECT().
		Update(gomock.Any(), "u1", "s1", gomock.AssignableToTypeOf(secret.UpdateReq{})).
		DoAndReturn(func(ctx context.Context, ownerID, secretID string, req secret.UpdateReq) (*secret.Secret, error) {
			// Title должен быть обрезан по краям: "  New Title  " -> "New Title"
			require.NotNil(t, req.Title)
			require.Equal(t, "New Title", *req.Title)
			require.Equal(t, 5, req.Version)
			require.Equal(t, map[string]any{"x": float64(1)}, req.Data)

			return &secret.Secret{
				ID:        "s1",
				OwnerID:   "u1",
				Type:      "note",
				Title:     *req.Title,
				Data:      map[string]any{"x": 1},
				Version:   6,
				CreatedAt: time.Unix(10, 0).UTC(),
				UpdatedAt: time.Unix(20, 0).UTC(),
			}, nil
		})

	h := NewUpdate(svc, testLogger(t))

	body := `{"title":"  New Title  ","data":{"x":1},"version":5}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/secrets/s1", strings.NewReader(body))
	rc := chi.NewRouteContext()
	rc.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	var got thttp.SecretResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "s1", got.ID)
	require.Equal(t, "New Title", got.Title)
	require.Equal(t, 6, got.Version)
}
