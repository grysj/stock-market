package service

import (
	"context"

	"remitly_task/internal/models"
	"remitly_task/internal/repository"
)

type Market struct {
	repo repository.Market
}

func NewMarket(repo repository.Market) *Market {
	return &Market{repo: repo}
}

func (s *Market) WalletStockOperation(ctx context.Context, input models.WalletStockOperation) error {
	if input.Type != "buy" && input.Type != "sell" {
		return repository.ErrInvalidOperation
	}

	return s.repo.ApplyWalletOperation(ctx, input.WalletID, input.StockName, input.Type)
}

func (s *Market) GetWallet(ctx context.Context, walletID string) (models.Wallet, error) {
	return s.repo.GetWallet(ctx, walletID)
}

func (s *Market) GetWalletStockQuantity(ctx context.Context, walletID string, stockName string) (int64, error) {
	return s.repo.GetWalletStockQuantity(ctx, walletID, stockName)
}

func (s *Market) GetStocks(ctx context.Context) ([]models.Stock, error) {
	return s.repo.GetBankStocks(ctx)
}

func (s *Market) SetStocks(ctx context.Context, input models.SetStocks) error {
	return s.repo.SetBankStocks(ctx, input.Stocks)
}

func (s *Market) GetLog(ctx context.Context) ([]models.AuditLogEntry, error) {
	return s.repo.GetAuditLog(ctx)
}
