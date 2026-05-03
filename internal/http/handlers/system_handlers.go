package handlers

import (
	"net/http"
	"os"
	"time"

	"remitly_task/internal/http/response"
)

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, response.Health{Status: "ok"})
}

func (h *Handlers) Chaos(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, response.Chaos{Message: "instance will terminate"})

	go func() {
		time.Sleep(10 * time.Millisecond)
		os.Exit(1)
	}()
}
