package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/models"
	"github.com/NailUsmanov/gophkeeper/internal/server/middlewares"

	thttp "github.com/NailUsmanov/gophkeeper/internal/server/transport/http"
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

// Test RegisterHandler
func TestNewRegister_Table(t *testing.T) {
	type want struct {
		status     int
		cookieName string
		cookieVal  string
	}

	type tc struct {
		name      string
		body      any // будет маршалиться в JSON, либо строка с кривым JSON
		setupMock func(m *MockAuthService)
		want      want
		// доп.проверки тела ответа
		assertBody func(t *testing.T, rr *httptest.ResponseRecorder)
	}

	fixedTime := time.Date(2025, 9, 9, 15, 4, 5, 0, time.UTC)

	tests := []tc{
		{
			name: "OK -> 201, JSON body, Set-Cookie auth_token",
			body: thttp.AuthRequest{
				Email:    "alice@example.com",
				Password: "secret123",
			},
			setupMock: func(m *MockAuthService) {
				m.EXPECT().
					Register(gomock.Any(), "alice@example.com", "secret123").
					Return(&models.User{
						ID:           "u-1",
						Email:        "alice@example.com",
						PasswordHash: "H:... (не уходит в ответ)",
						CreatedAt:    fixedTime,
					}, "tok-xyz", nil)
			},
			want: want{
				status:     http.StatusCreated,
				cookieName: "auth_token",
				cookieVal:  "tok-xyz",
			},
			assertBody: func(t *testing.T, rr *httptest.ResponseRecorder) {
				// ответ должен быть UserResponse (без PasswordHash)
				var got thttp.UserResponse
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
				require.Equal(t, "u-1", got.ID)
				require.Equal(t, "alice@example.com", got.Email)
				require.WithinDuration(t, fixedTime, got.CreatedAt, time.Second)
				require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
			},
		},
		{
			name: "Bad JSON -> 400",
			body: `{"email":"alice@example.com" "password": "oops"}`,
			setupMock: func(m *MockAuthService) { // сервис не должен вызываться
			},
			want: want{
				status: http.StatusBadRequest,
			},
		},
		{
			name: "AlreadyExists -> 409",
			body: thttp.AuthRequest{
				Email:    "bob@example.com",
				Password: "pw",
			},
			setupMock: func(m *MockAuthService) {
				// даже если доменная ошибка «AlreadyExists»,
				// текущий хендлер превращает её в Internal.
				m.EXPECT().
					Register(gomock.Any(), "bob@example.com", "pw").
					Return(nil, "", models.NewAlreadyExists(nil))
			},
			want: want{
				status: http.StatusConflict,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSvc := NewMockAuthService(ctrl)

			if tt.setupMock != nil {
				tt.setupMock(mockSvc)
			}

			h := NewRegister(mockSvc, testLogger(t))

			var reqBody *bytes.Reader
			switch v := tt.body.(type) {
			case string:
				reqBody = bytes.NewReader([]byte(v))
			default:
				b, err := json.Marshal(v)
				require.NoError(t, err)
				reqBody = bytes.NewReader(b)
			}
			req := httptest.NewRequest(http.MethodPost, "/register", reqBody)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h(w, req)

			// статус
			require.Equal(t, tt.want.status, w.Code)

			// cookie — только для успешного кейса
			if tt.want.cookieName != "" {
				res := w.Result()
				defer res.Body.Close()

				// найдём куку по имени
				var got *http.Cookie
				for _, c := range res.Cookies() {
					if c.Name == tt.want.cookieName {
						got = c
						break
					}
				}
				require.NotNil(t, got, "expected cookie %q", tt.want.cookieName)
				require.Equal(t, tt.want.cookieVal, got.Value)
				require.Equal(t, "/", got.Path)
				require.True(t, got.HttpOnly)
				// SameSite и Secure не всегда одинаково читаются из Recorder;
				// при желании можно парсить заголовок Set-Cookie строкой.
			}

			if tt.assertBody != nil {
				tt.assertBody(t, w)
			}

		})
	}

}

func TestNewLogin_Table(t *testing.T) {
	type want struct {
		status     int
		cookieName string
		cookieVal  string
	}

	type tc struct {
		name      string
		body      any // маршалим в JSON, либо отправим сырую строку (для битого JSON)
		setupMock func(m *MockAuthService)
		want      want
		// доп.проверки тела и заголовков
		assert func(t *testing.T, rr *httptest.ResponseRecorder)
	}

	fixedTime := time.Date(2025, 9, 9, 15, 4, 5, 0, time.UTC)

	tests := []tc{
		{
			name: "OK -> 200, JSON body, Set-Cookie auth_token",
			body: thttp.AuthRequest{
				Email:    "alice@example.com",
				Password: "secret",
			},
			setupMock: func(m *MockAuthService) {
				m.EXPECT().
					Login(gomock.Any(), "alice@example.com", "secret").
					Return(&models.User{
						ID:        "u-1",
						Email:     "alice@example.com",
						CreatedAt: fixedTime,
					}, "tok-login", nil)
			},
			want: want{
				status:     http.StatusOK,
				cookieName: "auth_token",
				cookieVal:  "tok-login",
			},
			assert: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var got thttp.UserResponse
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
				require.Equal(t, "u-1", got.ID)
				require.Equal(t, "alice@example.com", got.Email)
				require.WithinDuration(t, fixedTime, got.CreatedAt, time.Second)
				require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
			},
		},
		{
			name: "Bad JSON -> 500 (текущее поведение WriteError)",
			body: `{"email": "x" "password":"y"}`, // битый JSON (нет запятой)
			setupMock: func(m *MockAuthService) {
				// сервис не должен вызываться
			},
			want: want{
				status: http.StatusInternalServerError,
			},
		},
		{
			name: "Empty fields -> 500 (текущее поведение, т.к. WriteError получает обычный err)",
			body: thttp.AuthRequest{
				Email:    "",
				Password: "",
			},
			setupMock: func(m *MockAuthService) {},
			want:      want{status: http.StatusInternalServerError},
		},
		{
			name: "Unauthorized -> 401",
			body: thttp.AuthRequest{
				Email:    "bob@example.com",
				Password: "pw",
			},
			setupMock: func(m *MockAuthService) {
				m.EXPECT().
					Login(gomock.Any(), "bob@example.com", "pw").
					Return(nil, "", models.NewUnauthorized(nil)) // не важно какая — хендлер вернёт 500
			},
			want: want{
				status: http.StatusUnauthorized,
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSvc := NewMockAuthService(ctrl)
			if tc.setupMock != nil {
				tc.setupMock(mockSvc)
			}

			log := zap.NewNop().Sugar()
			h := NewLogin(mockSvc, log)

			var bodyReader *bytes.Reader
			switch v := tc.body.(type) {
			case string:
				bodyReader = bytes.NewReader([]byte(v))
			default:
				b, err := json.Marshal(v)
				require.NoError(t, err)
				bodyReader = bytes.NewReader(b)
			}

			req := httptest.NewRequest(http.MethodPost, "/login", bodyReader)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			h(rr, req)

			require.Equal(t, tc.want.status, rr.Code)

			// проверяем cookie только для успешного кейса
			if tc.want.cookieName != "" {
				var got *http.Cookie
				for _, c := range rr.Result().Cookies() {
					if c.Name == tc.want.cookieName {
						got = c
						break
					}
				}
				require.NotNil(t, got, "expected cookie %q", tc.want.cookieName)
				require.Equal(t, tc.want.cookieVal, got.Value)
				require.Equal(t, "/", got.Path)
				require.True(t, got.HttpOnly)
			}

			if tc.assert != nil {
				tc.assert(t, rr)
			}
		})
	}
}

func TestNewLogout_Table(t *testing.T) {
	type want struct {
		status          int
		expectClearCook bool
	}

	type tc struct {
		name      string
		cookieVal string // что положим клиентом в запрос
		setupMock func(m *MockAuthService)
		want      want
	}

	tests := []tc{
		{
			name:      "OK with token cookie -> revoke called, 204, cookie cleared",
			cookieVal: "tok-1",
			setupMock: func(m *MockAuthService) {
				m.EXPECT().
					Logout(gomock.Any(), "tok-1").
					Return(nil)
			},
			want: want{
				status:          http.StatusNoContent,
				expectClearCook: true,
			},
		},
		{
			name:      "Revoke error -> still 204, cookie cleared (идемпотентность)",
			cookieVal: "tok-2",
			setupMock: func(m *MockAuthService) {
				m.EXPECT().
					Logout(gomock.Any(), "tok-2").
					Return(models.NewInternal(nil))
			},
			want: want{
				status:          http.StatusNoContent,
				expectClearCook: true,
			},
		},
		{
			name:      "No cookie -> 204, server still sets clearing cookie",
			cookieVal: "",
			setupMock: func(m *MockAuthService) {
				// Logout не вызывается
			},
			want: want{
				status:          http.StatusNoContent,
				expectClearCook: true, // твой код всегда шлёт Set-Cookie с Max-Age=-1
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSvc := NewMockAuthService(ctrl)
			if tc.setupMock != nil {
				tc.setupMock(mockSvc)
			}

			log := zap.NewNop().Sugar()
			h := NewLogout(mockSvc, log)

			req := httptest.NewRequest(http.MethodPost, "/logout", nil)
			// если нужно — положим auth_token в запрос
			if tc.cookieVal != "" {
				req.AddCookie(&http.Cookie{
					Name:  "auth_token",
					Value: tc.cookieVal,
					Path:  "/",
				})
			}

			rr := httptest.NewRecorder()
			h(rr, req)

			require.Equal(t, tc.want.status, rr.Code)

			// сервер должен прислать Set-Cookie, который очищает куку
			if tc.want.expectClearCook {
				setCookies := rr.Result().Header.Values("Set-Cookie")
				require.NotEmpty(t, setCookies, "expected Set-Cookie header")

				// Проверим базовые признаки "очистили куку"
				found := false
				for _, line := range setCookies {
					// ожидаем: auth_token=; Max-Age=0 или -1 (у тебя MaxAge:-1)
					if bytes.Contains([]byte(line), []byte("auth_token=")) &&
						(bytes.Contains([]byte(line), []byte("Max-Age=0")) || bytes.Contains([]byte(line), []byte("Max-Age=-1"))) {
						found = true
						break
					}
				}
				require.True(t, found, "expected clearing auth_token cookie, got %v", setCookies)
			}
		})
	}
}
