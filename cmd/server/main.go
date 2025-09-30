// Package main это точка входа приложения.
// Парсит конфиг, готовит зависимости, конструирует app, запускает HTTP-сервер.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/server/app"
	"github.com/NailUsmanov/gophkeeper/pkg/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var (
	buildVersion string = "N/A"
	buildDate    string = "N/A"
	buildCommit  string = "N/A"
)

func main() {
	// Выдаем версию, дату и комментарий приложения.
	fmt.Printf("Build version: %s\nBuild date: %s\nBuild commit: %s\n", buildVersion, buildDate, buildCommit)

	go func() {
		log.Println("pprof listening on http://localhost:6060")
		_ = http.ListenAndServe("localhost:6060", nil)
	}()

	// Создаем предустановленный регистратор zap
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	sugar := logger.Sugar()

	// Создаем конфиг.
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Подключаемся к БД.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		sugar.Fatalw("pgxpool.New failed", "err", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		sugar.Fatalw("database ping failed", "err", err)
	}

	// Сборка приложения.
	application := app.NewApp(sugar, pool, cfg.AttachmentDir)

	// Делаем запуск с graceful shutdown
	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err := application.Run(runCtx, cfg.ServerAddr); err != nil {
		sugar.Fatalw("server run failed", "err", err)
	}
}
