package app_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/NailUsmanov/gophkeeper/internal/server/app"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// просто проверяем, что конструктор не паникует и роутер есть
func TestNewApp_Builds(t *testing.T) {
	logger := zap.NewNop().Sugar()
	baseDir := t.TempDir()
	var db *pgxpool.Pool // нулевое значение допустимо, пока мы не трогаем хендлеры

	a := app.NewApp(logger, db, baseDir)
	if a == nil {
		t.Fatal("NewApp returned nil")
	}
}

// запускаем сервер на свободном порту, делаем запрос и останавливаем через контекст
func TestApp_Run_StartsAndStops(t *testing.T) {
	logger := zap.NewNop().Sugar()
	baseDir := t.TempDir()
	var db *pgxpool.Pool

	a := app.NewApp(logger, db, baseDir)

	// выберем свободный порт
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- a.Run(ctx, addr) }()

	// подождём старта сервера и попробуем сделать запрос
	client := &http.Client{Timeout: 2 * time.Second}
	ok := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://" + addr + "/__no_such_route__")
		if err == nil {
			_ = resp.Body.Close()
			// 404 — норм, главное что сервер отвечает
			if resp.StatusCode == http.StatusNotFound {
				ok = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ok {
		t.Fatal("server did not start or did not respond in time")
	}

	// graceful shutdown
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down gracefully")
	}
}
