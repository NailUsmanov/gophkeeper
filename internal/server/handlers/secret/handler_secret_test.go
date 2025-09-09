package handlers

import (
	"context"
	"encoding/json"
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

// вспомогательная обёртка для логгера
func testLogger(t *testing.T) *zap.SugaredLogger {
	l, _ := zap.NewDevelopment()
	t.Cleanup(func() { _ = l.Sync() })
	return l.Sugar()
}

func FakeAuthMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, 1))
		next.ServeHTTP(w, r)
	})
}

// Test Create Hanlder
func TestCreateOK(t *testing.T) {
	t.Run("correct test", func(t *testing.T) {
		// Создаем контроллер, который следит за исполнением моков.
		// defer ctrl.Finish() проверяет в конце, что все вызовы моков были такими,
		// какими мы ожидали (EXPECT()).
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mockgen сервис
		mockSvc := NewMockCreateSecretService(ctrl)

		mockSvc.EXPECT().
			Create(
				gomock.Any(),
				"u1",
				gomock.AssignableToTypeOf(secret.CreateReq{Type: "note"}),
			).
			DoAndReturn(func(ctx context.Context, ownerID string, req secret.CreateReq) (*secret.Secret, error) {
				require.Equal(t, models.SecretType("note"), req.Type)
				require.Equal(t, "test", req.Title)
				require.Equal(t, map[string]any{"test123": "123"}, req.Data)
				return &secret.Secret{
					ID:        "s1",
					OwnerID:   "u1",
					Type:      "note",
					Title:     "test",
					Data:      map[string]any{"test123": "123"},
					Version:   1,
					CreatedAt: time.Unix(10, 0).UTC(),
					UpdatedAt: time.Unix(20, 0).UTC(),
				}, nil
			})

		h := NewCreateSecret(mockSvc, testLogger(t))

		body := `{"type":"note","title":"test","data":{"test123":"123"},"version":1}`
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
		require.Equal(t, "test", got.Title)
		require.Equal(t, 1, got.Version)
		require.Contains(t, got.Data, "test123")
		require.Equal(t, "123", got.Data["test123"])

	})
}

func TestCreateInvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockCreateSecretService(ctrl)

	// сервис вызываться не должен
	h := NewCreateSecret(mockSvc, testLogger(t))

	r := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader(`{`))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "error")
}

func TestCreateMissingFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockCreateSecretService(ctrl)
	// сервис вызываться не должен (валидация на уровне хендлера)
	h := NewCreateSecret(mockSvc, testLogger(t))

	body := `{"type":"note","title":"","data":{}}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "error")

}

func TestCreateUnauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockCreateSecretService(ctrl)

	// сервис вызываться не должен (нет userID)
	h := NewCreateSecret(mockSvc, testLogger(t))

	body := `"type":"note","title":"t","data":{"x":1}}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader(body))
	// userID НЕ кладём в контекст

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), "error")
}

func TestCreateInternal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockCreateSecretService(ctrl)
	mockSvc.
		EXPECT().
		Create(gomock.Any(),
			"u1",
			gomock.AssignableToTypeOf(secret.CreateReq{}),
		).
		Return(nil, models.NewInternal(nil)) // сервис отдал внутреннюю ошибку

	h := NewCreateSecret(mockSvc, testLogger(t))

	body := `{"type":"note","title":"boom","data":{"k":"v"}}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", strings.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "error")
}

// Test GetByID Handler
func TestGetByIDOK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockGetSecretService(ctrl)

	mockSvc.EXPECT().
		GetByID(
			gomock.Any(),
			"u1",
			"s1",
		).DoAndReturn(func(ctx context.Context, ownerID, secretID string) (*secret.Secret, error) {
		return &secret.Secret{
			ID:        "s1",
			OwnerID:   "u1",
			Type:      "note",
			Title:     "test",
			Data:      map[string]any{"test123": "123"},
			Version:   3,
			CreatedAt: time.Unix(10, 0).UTC(),
			UpdatedAt: time.Unix(20, 0).UTC(),
		}, nil
	})

	h := NewGetByID(mockSvc, testLogger(t))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/s1", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var got thttp.SecretResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "s1", got.ID)
	require.Equal(t, "note", got.Type)
	require.Equal(t, "test", got.Title)
	require.Equal(t, 3, got.Version)
	_, err := time.Parse(time.RFC3339, got.CreatedAt)
	require.NoError(t, err)
	_, err = time.Parse(time.RFC3339, got.UpdatedAt)
	require.NoError(t, err)
}

func TestGetByIDUnauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockGetSecretService(ctrl)

	h := NewGetByID(mockSvc, testLogger(t))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/s1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	// userID не кладем

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), "error")
}

func TestGetByIDNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockGetSecretService(ctrl)

	mockSvc.EXPECT().
		GetByID(gomock.Any(), "u1", "missing").Return(nil, models.NewNotFound(nil))

	h := NewGetByID(mockSvc, testLogger(t))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/missing", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "missing")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "error")
}

// Test List Handler
func TestListOK(t *testing.T) {
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
			{
				ID:        "s2",
				OwnerID:   "u1",
				Type:      "password",
				Title:     "B",
				Data:      map[string]any{"login": "x"},
				Version:   2,
				CreatedAt: time.Unix(10, 0).UTC(),
				UpdatedAt: time.Unix(20, 0).UTC(),
			},
		}, 2, nil)

	h := NewList(mockSvc, testLogger(t))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets", nil)
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var got thttp.ListSecretResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, 20, got.Limit)
	require.Equal(t, 0, got.Offset)
	require.Equal(t, 2, got.Total)
	require.Len(t, got.Items, 2)
	require.Equal(t, "s1", got.Items[0].ID)
	require.Equal(t, "note", got.Items[0].Type)
}

func TestListInvalidLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockGetSecretService(ctrl)

	h := NewList(mockSvc, testLogger(t))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets?limit=0", nil)
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"error"`)
}

func TestListInvalidOffset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockGetSecretService(ctrl)

	h := NewList(mockSvc, testLogger(t))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets?offset=-1", nil)
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "error")
}

func TestListInvalidType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewList(NewMockGetSecretService(ctrl), testLogger(t))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets?type=weird", nil)
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"error"`)
}

func TestListUnauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewList(NewMockGetSecretService(ctrl), testLogger(t))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets", nil)
	// userID НЕ кладём
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), "error")
}
func TestList_ServiceInternal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockGetSecretService(ctrl)
	mockSvc.
		EXPECT().
		List(gomock.Any(), "u1", 20, 0, secret.SecretListFilter{}).
		Return(nil, 0, models.NewInternal(nil))

	h := NewList(mockSvc, testLogger(t))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/secrets", nil)
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), `"error"`)
}

// Test Update Handler
func TestNewUpdateOK(t *testing.T) {

	t.Run("correct test", func(t *testing.T) {
		// Создаем контроллер, который следит за исполнением моков.
		// defer ctrl.Finish() проверяет в конце, что все вызовы моков были такими,
		// какими мы ожидали (EXPECT()).
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mockgen сервис
		mockSvc := NewMockUpdateService(ctrl)

		// ожидание вызова с любым контекстом, ownerID "u1", secretID "s1" и конкретным req

		mockSvc.EXPECT().Update(gomock.Any(), "u1", "s1", gomock.AssignableToTypeOf(secret.UpdateReq{})).DoAndReturn(
			func(ctx context.Context, ownerID, secretID string, req secret.UpdateReq) (*secret.Secret, error) {
				require.Equal(t, "u1", ownerID)
				require.Equal(t, "s1", secretID)
				require.Nil(t, req.Title)
				require.Equal(t, 3, req.Version)

				// КЛЮЧЕВОЕ: число пришло как float64 из JSON
				require.Contains(t, req.Data, "x")
				require.IsType(t, float64(0), req.Data["x"])
				require.Equal(t, float64(1), req.Data["x"])

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

		// готовим запрос с path-параметром id
		body := `{"data":{"x":1},"version":3}`
		r := httptest.NewRequest(http.MethodPut, "/api/v1/secrets/s1", strings.NewReader(body))

		// подсовываем chi.RouteContext с параметром id
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "s1")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		// и userID от middleware
		r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		require.Equal(t, http.StatusOK, w.Code)

		var got thttp.SecretResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
		require.Equal(t, "s1", got.ID)
		require.Equal(t, "note", got.Type)
		require.Equal(t, 4, got.Version)
	})
}

func TestNewUpdateServiceInvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockUpdateService(ctrl)
	// сервис НЕ должен вызываться при невалидном JSON
	h := NewUpdate(mockSvc, testLogger(t))

	r := httptest.NewRequest(http.MethodPut, "/api/v1/secrets/s1", strings.NewReader(`{`))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	// WriteError для Validation возвращает 422
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Body.String(), `"error"`)
}

func TestNewUpdatedServiceConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := NewMockUpdateService(ctrl)

	mockSvc.
		EXPECT().
		Update(gomock.Any(), "u1", "s1", gomock.AssignableToTypeOf(secret.UpdateReq{})).
		DoAndReturn(func(ctx context.Context, ownerID, secretID string, req secret.UpdateReq) (*secret.Secret, error) {
			require.Equal(t, "u1", ownerID)
			require.Equal(t, "s1", secretID)
			require.Equal(t, 3, req.Version)
			require.Contains(t, req.Data, "x")
			require.IsType(t, float64(0), req.Data["x"]) // важно!
			return nil, models.NewConflict(nil)
		})

	h := NewUpdate(mockSvc, testLogger(t))

	r := httptest.NewRequest(http.MethodPut, "/api/v1/secrets/s1", strings.NewReader(`{"data":{"x":1},"version":3}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "s1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r = r.WithContext(context.WithValue(r.Context(), middlewares.UserLoginKey, "u1"))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusConflict, w.Code)
}
