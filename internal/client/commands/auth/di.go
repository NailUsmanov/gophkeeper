// Package commands — слой CLI-команд клиента GophKeeper.
//
// commands/auth отвечает за регистрацию, логирование и завершение сессии пользователя.
package commands_auth

import (
	"github.com/NailUsmanov/gophkeeper/internal/client/session"
	"github.com/NailUsmanov/gophkeeper/internal/client/transport"
)

// Эти переменные переопределим в тестах.
var (
	saveToken    = session.SaveToken
	clearSession = session.Clear
	loadToken    = session.LoadToken

	// используем в httptest.Server
	newClient = transport.NewClient
)
