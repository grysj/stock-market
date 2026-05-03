package main

import (
	"log"

	"remitly_task/internal/app"
	"remitly_task/internal/config"
)

func main() {
	cfg := config.Load()
	if err := app.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
