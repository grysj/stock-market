package handlers

import (
	"net/http"

	"remitly_task/internal/models"
	"remitly_task/internal/http/request"
	"remitly_task/internal/http/response"
)

func (h *Handlers) GetStocks(w http.ResponseWriter, r *http.Request) {
	stocks, err := h.market.GetStocks(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response.Error{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, response.Stocks{Stocks: stocks})
}

func (h *Handlers) SetStocks(w http.ResponseWriter, r *http.Request) {
	var req request.SetStocks
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, response.Error{Error: err.Error()})
		return
	}

	for _, stock := range req.Stocks {
		if stock.Name == "" || stock.Quantity < 0 {
			writeJSON(w, http.StatusBadRequest, response.Error{Error: "each stock must have non-empty name and non-negative quantity"})
			return
		}
	}

	if err := h.market.SetStocks(r.Context(), models.SetStocks{Stocks: req.Stocks}); err != nil {
		writeJSON(w, http.StatusInternalServerError, response.Error{Error: "internal server error"})
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) GetLog(w http.ResponseWriter, r *http.Request) {
	logEntries, err := h.market.GetLog(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response.Error{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, response.Log{Log: logEntries})
}
