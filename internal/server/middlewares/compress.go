// Package middlewares содержит middleware-функции для HTTP-сервера.

// Включает логирование запросов, сжатие ответов и аутентификацию пользователей.
package middlewares

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// CompressWriter — обёртка над http.ResponseWriter, выполняющая сжатие ответа в формате gzip.
type CompressWriter struct {
	w  http.ResponseWriter // поле для хранения оригинального ResponseWriter (куда обычно пишет сервер).
	zw *gzip.Writer        // gzip-писатель, который будет сжимать данные перед отправкой в w.
}

// NewCompressWriter создаёт новый CompressWriter с включённым gzip-сжатием.
func NewCompressWriter(w http.ResponseWriter) *CompressWriter {
	return &CompressWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

// Header возвращает заголовки HTTP-ответа.
func (c *CompressWriter) Header() http.Header {
	return c.w.Header()
}

// Write сжимает и записывает данные в тело HTTP-ответа.
func (c *CompressWriter) Write(p []byte) (int, error) {
	if c.Header().Get("Content-Encoding") == "" {
		c.Header().Del("Content-Length")
		c.Header().Set("Content-Encoding", "gzip")
	}
	return c.zw.Write(p)
}

// WriteHeader устанавливает статус-код HTTP-ответа и добавляет заголовок Content-Encoding.
func (c *CompressWriter) WriteHeader(statusCode int) {
	if c.Header().Get("Content-Encoding") == "" {
		c.Header().Del("Content-Length")
		c.Header().Set("Content-Encoding", "gzip")
	}
	c.w.WriteHeader(statusCode)
}

// Close завершает работу gzip.Writer и освобождает ресурсы.
// gzip записывает «хвост» потока только при закрытии.
// Без этого клиент получит «обрезанный» поток.
func (c *CompressWriter) Close() error {
	return c.zw.Close()
}

// CompressReader - обертка над http.Reader для чтения запроса , сжатого в формате gzip (распаковка входящего запроса).
type CompressReader struct {
	r  io.ReadCloser // оригинальное тело запроса (байты в gzip)
	zr *gzip.Reader  // распаковщик, который будет выдавать уже распакованные байты.
}

// NewComressReader создает новый CompressReader и инициализирует gzip.Reader.
func NewCompressReader(r io.ReadCloser) (*CompressReader, error) {
	zr, err := gzip.NewReader(r) // создаём новый gzip-декомпрессор.
	// Если ошибка — возвращаем её (например, если тело не было реально в gzip).
	if err != nil {
		return nil, err
	}
	return &CompressReader{
		r:  r,
		zr: zr,
	}, nil
}

// Read считывает и распаковывает данные из gzip-сжатого тела запроса.
func (cr *CompressReader) Read(p []byte) (int, error) {
	return cr.zr.Read(p)
}

// Close завершает работу gzip.Writer и освобождает ресурсы.
func (cr *CompressReader) Close() error {
	_ = cr.zr.Close() // закрываем gzip.Reader.
	return cr.r.Close()
}

// GzipMiddleware — HTTP middleware, распаковывающее входящие gzip-запросы и сжимающее ответы, если клиент поддерживает gzip.
func GzipMiddleware(next http.Handler) http.Handler {
	// оборачиваем стандартную функцию в Handler.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Распаковка входящих данных
		// Проверяем заголовок Content-Encoding: если в запросе клиент сказал «gzip» → тело зазиповано.
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			cr, err := NewCompressReader(r.Body) // Создаём CompressReader для распаковки.
			if err != nil {
				http.Error(w, "failed to decompress gzip body", http.StatusBadRequest)
				return
			}
			defer cr.Close()
			// Подменяем r.Body = cr — теперь все хендлеры ниже будут читать распакованное тело.
			r.Body = cr

			// Нормализуем тип, если пришёл «архивный» маркер
			if r.Header.Get("Content-Type") == "application/x-gzip" {
				r.Header.Set("Content-Type", "text/plain")
			}
		}

		// Подготовка сжатия исходящих данных
		// Проверяем заголовок Accept-Encoding: если клиент поддерживает gzip, то можно сжимать ответ.
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			cw := NewCompressWriter(w)
			// Важно закрыть gzip-поток.
			defer cw.Close()
			next.ServeHTTP(cw, r)
			return
		}
		// Если клиент не поддерживает gzip, просто вызываем следующий хендлер «как есть», без сжатия.
		next.ServeHTTP(w, r)
	})
}
