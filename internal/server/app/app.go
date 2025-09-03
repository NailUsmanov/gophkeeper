// Package app конфигурирет и запускает HTTP-приложение.

// Настраивает маршруты, middlewares и запускает сервер.

package app

import (
	"context"
	"net/http"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/server/middlewares"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// App инкапсулирует конфигурацию HTTP-сервера.
// Пока держим только роутер и логгер. Хранилище/handlers подключим позже.
// Такой скелет позволит запустить сервер и корректно его останавливать.

type App struct {
	router *chi.Mux
	sugar  *zap.SugaredLogger
}

// NewApp создает и настраивает экземлпяр Арр.
// Здесь регистрируем middleware/маршруты (пока заглушки).
func NewApp(sugar *zap.SugaredLogger) *App {
	r := chi.NewRouter()
	a := &App{
		router: r,
		sugar:  sugar,
	}
	a.setupRoutes()
	return a
}

// setupRoutes — здесь будут маршруты и middleware.
func (a *App) setupRoutes() {

	a.router.Use(middlewares.LoggingMiddleware(a.sugar))
	a.router.Use(middlewares.GzipMiddleware)
	a.router.Use(middlewares.AuthMiddleWare)
	// TODO: зарегистрировать хендлеры, когда они появятся:
	// a.router.Post("/register", handlers.Register()) // Регистрация
	// a.router.Post("/auth", handlers.Authentication()) // Аутентификация
	// a.router.Post("/login", handlers.Authorization()) // Авторизация
	// a.router.Get("/get", handlers.Get()) // Получение приватных данных
}

// Run запускает HTTP-сервер на указанном адресе и корректно завершает его по ctx.
func (a *App) Run(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: a.router,
	}

	// Graceful Shutdown
	go func() {
		<-ctx.Done()
		a.sugar.Infow("Shutting down server...")
		sdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sdCtx)
	}()

	err := srv.ListenAndServe()
	if err != nil {
		return err
	}
	return nil
}
