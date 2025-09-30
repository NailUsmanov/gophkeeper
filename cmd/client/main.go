// Package main - точка входа CLI-клиента для GophKeeper.
// Передает управление слою команд.
package main

import (
	"log"

	"github.com/NailUsmanov/gophkeeper/internal/client/commands"
)

var (
	buildVersion string = "N/A"
	buildDate    string = "N/A"
	buildCommit  string = "N/A"
)

func main() {
	if err := commands.Execute(buildVersion, buildDate, buildCommit); err != nil {
		log.Fatal(err)
	}
}
