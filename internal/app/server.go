package app

import (
	"net/http"

	"remitly_task/internal/config"
	apiHandlers "remitly_task/internal/http/handlers"
	"remitly_task/internal/http/router"
	"remitly_task/internal/repository/postgres"
	"remitly_task/internal/service"
)

func Run(cfg config.Config) error {
	store, err := postgres.NewStore(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	marketService := service.NewMarket(store)
	handler := apiHandlers.New(marketService)
	mux := router.New(handler)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	return server.ListenAndServe()
}
