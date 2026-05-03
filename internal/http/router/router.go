package router

import (
	"log"
	"net/http"

	"remitly_task/internal/http/handlers"
)

func New(h *handlers.Handlers) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("POST /chaos", h.Chaos)
	mux.HandleFunc("GET /stocks", h.GetStocks)
	mux.HandleFunc("POST /stocks", h.SetStocks)
	mux.HandleFunc("GET /log", h.GetLog)
	mux.HandleFunc("GET /wallets/{wallet_id}", h.GetWallet)
	mux.HandleFunc("GET /wallets/{wallet_id}/stocks/{stock_name}", h.GetWalletStock)
	mux.HandleFunc("POST /wallets/{wallet_id}/stocks/{stock_name}", h.WalletStockOperation)

	return withRequestLogging(mux)
}

func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("[%d] %s", sw.statusCode, r.URL.Path)
	})
}

type statusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
