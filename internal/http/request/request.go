package request

import "remitly_task/internal/models"

type WalletStockOperation struct {
	Type string `json:"type"`
}

type SetStocks struct {
	Stocks []models.Stock `json:"stocks"`
}
