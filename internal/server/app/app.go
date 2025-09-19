// Package app конфигурирет и запускает HTTP-приложение.

// Настраивает роутер chi, middlewares, производит инициализацию менеджера токенов
// и запускает/завершает сервер.

package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/security/password"
	"github.com/NailUsmanov/gophkeeper/internal/security/token"
	"github.com/jackc/pgx/v5/pgxpool"

	handler_attachment "github.com/NailUsmanov/gophkeeper/internal/server/handlers/attachment"
	handler_auth "github.com/NailUsmanov/gophkeeper/internal/server/handlers/auth"
	handler_secret "github.com/NailUsmanov/gophkeeper/internal/server/handlers/secret"
	"github.com/NailUsmanov/gophkeeper/internal/server/middlewares"
	"github.com/NailUsmanov/gophkeeper/internal/server/service/attachment"
	"github.com/NailUsmanov/gophkeeper/internal/server/service/auth"
	"github.com/NailUsmanov/gophkeeper/internal/server/service/secret"
	"github.com/NailUsmanov/gophkeeper/internal/server/storage/fs"
	postgres_attachment "github.com/NailUsmanov/gophkeeper/internal/server/storage/postgres/attachment"
	postgres_auth "github.com/NailUsmanov/gophkeeper/internal/server/storage/postgres/auth"
	postgres_secret "github.com/NailUsmanov/gophkeeper/internal/server/storage/postgres/secret"
	"github.com/NailUsmanov/gophkeeper/internal/server/storage/session/memory"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// App инкапсулирует конфигурацию HTTP-сервера.
// На текущем этапе содержит роутер и логгер.

type App struct {
	router  *chi.Mux
	sugar   *zap.SugaredLogger
	db      *pgxpool.Pool
	baseDir string
}

// NewApp создает и настраивает экземлпяр Арр.
// Здесь регистрируем middleware/маршруты (пока заглушки).
func NewApp(sugar *zap.SugaredLogger, db *pgxpool.Pool, baseDir string) *App {
	r := chi.NewRouter()
	a := &App{
		router:  r,
		sugar:   sugar,
		db:      db,
		baseDir: baseDir,
	}
	a.setupRoutes()
	return a
}

// setupRoutes регистрирует middleware и HTTP-маршруты.
func (a *App) setupRoutes() {

	// 1) Готовим менеджер токенов (opaque) и стор сессии.
	store := memory.NewStore()
	tm := token.NewOpaqueManager(store, 24*time.Hour)

	// 2) сервис Секрета.
	repo := postgres_secret.NewSecretRepo(a.db)
	svcSecret := secret.NewService(repo)

	// 3) сервис Регистрации.
	hasher := password.NewBcryptHasher(12)
	repoAuth := postgres_auth.NewUserRepository(a.db)
	svcAuth := auth.NewAuthService(repoAuth, hasher, tm)

	// 4) сервис Файлов.
	repoAttachment := postgres_attachment.NewAttachmentRepository(a.db)
	storageAttachment, err := fs.NewAttachmentStorage(a.baseDir)
	if err != nil {
		log.Fatal(err)
	}
	svcAttachment := attachment.NewAttachmentService(repoAttachment, storageAttachment, a.sugar)
	// 5) базовые мидлвари.
	a.router.Use(middlewares.LoggingMiddleware(a.sugar))
	a.router.Use(middlewares.GzipMiddleware)

	// 6) Публичные endpoints: регистрация/логин/health и т.д.
	a.router.Post("/api/v1/register", handler_auth.NewRegister(svcAuth, a.sugar))
	a.router.Post("/api/v1/login", handler_auth.NewLogin(svcAuth, a.sugar))
	a.router.Post("/api/v1/logout", handler_auth.NewLogout(svcAuth, a.sugar))
	a.router.Get("/ping", handler_attachment.NewPing(svcAttachment, a.sugar))

	// 7) Защищённые маршруты — только с валидным токеном
	a.router.Group(func(r chi.Router) {
		r.Use(middlewares.AuthMiddleWare(tm)) // tm реализует Validate
		r.Post("/api/v1/secrets", handler_secret.NewCreateSecret(svcSecret, a.sugar))
		r.Get("/api/v1/secrets/{id}", handler_secret.NewGetByID(svcSecret, a.sugar))
		r.Get("/api/v1/secrets", handler_secret.NewList(svcSecret, a.sugar))
		r.Put("/api/v1/secrets/{id}", handler_secret.NewUpdate(svcSecret, a.sugar))

		r.Post("/api/v1/attachments", handler_attachment.NewUpload(svcAttachment, a.sugar))
		r.Get("/api/v1/attachments/{id}", handler_attachment.NewDownload(svcAttachment, a.sugar))
		r.Get("/api/v1/attachments", handler_attachment.NewListAttachments(svcAttachment, a.sugar))
	})
}

// Run запускает HTTP-сервер на указанном адресе и корректно завершает его по ctx.
func (a *App) Run(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           a.router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Фоновая горутина ждёт завершения контекста и мягко останавливает сервер(GracefulShutdown).
	go func() {
		<-ctx.Done()
		a.sugar.Infow("Shutting down server...")
		sdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sdCtx)
	}()

	// Запуск: если это штатное закрытие — не считаем ошибкой.
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
