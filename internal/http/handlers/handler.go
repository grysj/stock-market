package handlers

import (
	"context"

	"remitly_task/internal/models"
)

type MarketService interface {
	WalletStockOperation(ctx context.Context, input models.WalletStockOperation) error
	GetWallet(ctx context.Context, walletID string) (models.Wallet, error)
	GetWalletStockQuantity(ctx context.Context, walletID string, stockName string) (int64, error)
	GetStocks(ctx context.Context) ([]models.Stock, error)
	SetStocks(ctx context.Context, input models.SetStocks) error
	GetLog(ctx context.Context) ([]models.AuditLogEntry, error)
}

type Handlers struct {
	market MarketService
}

func New(market MarketService) *Handlers {
	return &Handlers{market: market}
}
