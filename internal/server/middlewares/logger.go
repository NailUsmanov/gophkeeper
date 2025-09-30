// Package middlewares содержит middleware-функции для HTTP-сервера.
// Здесь реализуются:
//   - логирование запросов;
//   - сжатие ответов (gzip);
//   - аутентификация пользователей через cookie.
//
// LoggingMiddleware осуществляет логировение запросов.

package middlewares

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// responseData содержит данные об ответе HTTP-сервере - размер в байтах и статус-код.

// Используется для захвата информации, которую нельзя получить напрямую из ResponseWritter.
type responseData struct {
	size   int
	status int
}

// Обертка над стандартным http.ResponseWriter, чтобы перехватывать вызовы Write и WriteHeader,
// затем сохранять данные (размер и статус) в responseData и делегировать вызовы
// оригинальному ResponseWriter
type loggingResponseWriter struct {
	http.ResponseWriter
	responseData *responseData
}

// loggingResponseWritter.WriteHeader устанавливает статус код HTTP-ответа.
func (r *loggingResponseWriter) Write(p []byte) (int, error) {
	size, err := r.ResponseWriter.Write(p)
	r.responseData.size += size // здесь мы перехватываем размер ответа в байтах
	return size, err
}

// loggingResponseWriter.WriteHeader устанавливает статут код HTTP-ответа.
func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode // записываем статус-код
}

// LoggingMiddleware возвращает middleware, логирующее HTTP-запросы.

// В лог записывается URI, метод, статус-код, размер ответа и длительность обработки.
func LoggingMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			respData := responseData{
				status: http.StatusOK,
				size:   0,
			}

			lw := loggingResponseWriter{
				ResponseWriter: w,
				responseData:   &respData,
			}
			next.ServeHTTP(&lw, r)

			duration := time.Since(start)
			logger.Infow("http_request",
				"uri", r.RequestURI,
				"method", r.Method,
				"status", respData.status,
				"size", respData.size,
				"duration", duration,
			)
		})

	}
}
