package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"remitly_task/internal/http/request"
	"remitly_task/internal/http/response"
	"remitly_task/internal/models"
	"remitly_task/internal/repository"
)

func (h *Handlers) WalletStockOperation(w http.ResponseWriter, r *http.Request) {
	walletID := r.PathValue("wallet_id")
	stockName := r.PathValue("stock_name")

	var req request.WalletStockOperation
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error{Error: err.Error()})
		return
	}

	err := h.market.WalletStockOperation(r.Context(), models.WalletStockOperation{
		WalletID:  walletID,
		StockName: stockName,
		Type:      req.Type,
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrInvalidOperation):
			writeJSON(w, http.StatusBadRequest, response.Error{Error: err.Error()})
		case errors.Is(err, repository.ErrStockNotFound):
			writeJSON(w, http.StatusNotFound, response.Error{Error: err.Error()})
		case errors.Is(err, repository.ErrNoBankStock), errors.Is(err, repository.ErrNoWalletStock):
			writeJSON(w, http.StatusBadRequest, response.Error{Error: err.Error()})
		default:
			writeJSON(w, http.StatusInternalServerError, response.Error{Error: "internal server error"})
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) GetWallet(w http.ResponseWriter, r *http.Request) {
	walletID := r.PathValue("wallet_id")

	wallet, err := h.market.GetWallet(r.Context(), walletID)
	if err != nil {
		if errors.Is(err, repository.ErrWalletNotFound) {
			writeJSON(w, http.StatusNotFound, response.Error{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, response.Error{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, wallet)
}

func (h *Handlers) GetWalletStock(w http.ResponseWriter, r *http.Request) {
	walletID := r.PathValue("wallet_id")
	stockName := r.PathValue("stock_name")

	quantity, err := h.market.GetWalletStockQuantity(r.Context(), walletID, stockName)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrWalletNotFound), errors.Is(err, repository.ErrStockNotFound):
			writeJSON(w, http.StatusNotFound, response.Error{Error: err.Error()})
		default:
			writeJSON(w, http.StatusInternalServerError, response.Error{Error: "internal server error"})
		}
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "%d", quantity)
}
