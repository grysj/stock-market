package repository

import (
	"context"
	"errors"

	"remitly_task/internal/models"
)

var (
	ErrStockNotFound    = errors.New("stock not found")
	ErrWalletNotFound   = errors.New("wallet not found")
	ErrNoBankStock      = errors.New("bank has no stock available")
	ErrNoWalletStock    = errors.New("wallet has no stock available")
	ErrInvalidOperation = errors.New("invalid operation type")
)

type Market interface {
	ApplyWalletOperation(ctx context.Context, walletID string, stockName string, operation string) error
	GetWallet(ctx context.Context, walletID string) (models.Wallet, error)
	GetWalletStockQuantity(ctx context.Context, walletID string, stockName string) (int64, error)
	GetBankStocks(ctx context.Context) ([]models.Stock, error)
	SetBankStocks(ctx context.Context, stocks []models.Stock) error
	GetAuditLog(ctx context.Context) ([]models.AuditLogEntry, error)
}
