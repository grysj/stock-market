package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"remitly_task/internal/models"
	"remitly_task/internal/repository"
	"remitly_task/internal/service"
)

// mockRepo implements repositories.Market for service layer tests.
type mockRepo struct {
	applyWalletOperationFn   func(ctx context.Context, walletID, stockName, operation string) error
	getWalletFn              func(ctx context.Context, walletID string) (models.Wallet, error)
	getWalletStockQuantityFn func(ctx context.Context, walletID, stockName string) (int64, error)
	getBankStocksFn          func(ctx context.Context) ([]models.Stock, error)
	setBankStocksFn          func(ctx context.Context, stocks []models.Stock) error
	getAuditLogFn            func(ctx context.Context) ([]models.AuditLogEntry, error)
}

func (m *mockRepo) ApplyWalletOperation(ctx context.Context, walletID, stockName, operation string) error {
	if m.applyWalletOperationFn != nil {
		return m.applyWalletOperationFn(ctx, walletID, stockName, operation)
	}
	return nil
}

func (m *mockRepo) GetWallet(ctx context.Context, walletID string) (models.Wallet, error) {
	if m.getWalletFn != nil {
		return m.getWalletFn(ctx, walletID)
	}
	return models.Wallet{ID: walletID}, nil
}

func (m *mockRepo) GetWalletStockQuantity(ctx context.Context, walletID, stockName string) (int64, error) {
	if m.getWalletStockQuantityFn != nil {
		return m.getWalletStockQuantityFn(ctx, walletID, stockName)
	}
	return 0, nil
}

func (m *mockRepo) GetBankStocks(ctx context.Context) ([]models.Stock, error) {
	if m.getBankStocksFn != nil {
		return m.getBankStocksFn(ctx)
	}
	return nil, nil
}

func (m *mockRepo) SetBankStocks(ctx context.Context, stocks []models.Stock) error {
	if m.setBankStocksFn != nil {
		return m.setBankStocksFn(ctx, stocks)
	}
	return nil
}

func (m *mockRepo) GetAuditLog(ctx context.Context) ([]models.AuditLogEntry, error) {
	if m.getAuditLogFn != nil {
		return m.getAuditLogFn(ctx)
	}
	return nil, nil
}

func TestService_WalletStockOperation(t *testing.T) {
	t.Run("rejects invalid operation types", func(t *testing.T) {
		tests := []struct {
			name    string
			opType  string
		}{
			{"unknown type", "hold"},
			{"empty type", ""},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				called := false
				svc := service.NewMarket(&mockRepo{
					applyWalletOperationFn: func(_ context.Context, _, _, _ string) error {
						called = true
						return nil
					},
				})
				err := svc.WalletStockOperation(context.Background(), models.WalletStockOperation{
					WalletID: "w1", StockName: "AAPL", Type: tt.opType,
				})
				require.ErrorIs(t, err, repository.ErrInvalidOperation)
				assert.False(t, called, "repo should not be called for invalid operation type")
			})
		}
	})

	t.Run("forwards valid operations to repo", func(t *testing.T) {
		tests := []struct {
			name   string
			opType string
		}{
			{"buy", "buy"},
			{"sell", "sell"},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				var gotWalletID, gotStockName, gotOp string
				svc := service.NewMarket(&mockRepo{
					applyWalletOperationFn: func(_ context.Context, walletID, stockName, operation string) error {
						gotWalletID, gotStockName, gotOp = walletID, stockName, operation
						return nil
					},
				})
				err := svc.WalletStockOperation(context.Background(), models.WalletStockOperation{
					WalletID: "wallet1", StockName: "AAPL", Type: tt.opType,
				})
				require.NoError(t, err)
				assert.Equal(t, "wallet1", gotWalletID)
				assert.Equal(t, "AAPL", gotStockName)
				assert.Equal(t, tt.opType, gotOp)
			})
		}
	})

	t.Run("propagates repo errors", func(t *testing.T) {
		tests := []struct {
			name    string
			repoErr error
		}{
			{"ErrStockNotFound", repository.ErrStockNotFound},
			{"ErrNoBankStock", repository.ErrNoBankStock},
			{"ErrNoWalletStock", repository.ErrNoWalletStock},
			{"unexpected db error", errors.New("unexpected db error")},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				svc := service.NewMarket(&mockRepo{
					applyWalletOperationFn: func(_ context.Context, _, _, _ string) error {
						return tt.repoErr
					},
				})
				err := svc.WalletStockOperation(context.Background(), models.WalletStockOperation{
					WalletID: "w1", StockName: "AAPL", Type: "buy",
				})
				assert.ErrorIs(t, err, tt.repoErr)
			})
		}
	})
}

func TestService_GetWallet(t *testing.T) {
	t.Run("returns wallet from repo", func(t *testing.T) {
		expected := models.Wallet{
			ID:     "w1",
			Stocks: []models.Stock{{Name: "AAPL", Quantity: 5}},
		}
		svc := service.NewMarket(&mockRepo{
			getWalletFn: func(_ context.Context, _ string) (models.Wallet, error) {
				return expected, nil
			},
		})

		wallet, err := svc.GetWallet(context.Background(), "w1")
		require.NoError(t, err)
		assert.Equal(t, "w1", wallet.ID)
		require.Len(t, wallet.Stocks, 1)
		assert.Equal(t, "AAPL", wallet.Stocks[0].Name)
	})

	t.Run("propagates not found error", func(t *testing.T) {
		svc := service.NewMarket(&mockRepo{
			getWalletFn: func(_ context.Context, _ string) (models.Wallet, error) {
				return models.Wallet{}, repository.ErrWalletNotFound
			},
		})

		_, err := svc.GetWallet(context.Background(), "missing")
		assert.ErrorIs(t, err, repository.ErrWalletNotFound)
	})
}

func TestService_GetWalletStockQuantity(t *testing.T) {
	t.Run("returns quantity from repo", func(t *testing.T) {
		svc := service.NewMarket(&mockRepo{
			getWalletStockQuantityFn: func(_ context.Context, _, _ string) (int64, error) {
				return 42, nil
			},
		})

		qty, err := svc.GetWalletStockQuantity(context.Background(), "w1", "AAPL")
		require.NoError(t, err)
		assert.Equal(t, int64(42), qty)
	})

	t.Run("propagates repo errors", func(t *testing.T) {
		tests := []struct {
			name    string
			repoErr error
		}{
			{"ErrWalletNotFound", repository.ErrWalletNotFound},
			{"ErrStockNotFound", repository.ErrStockNotFound},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				svc := service.NewMarket(&mockRepo{
					getWalletStockQuantityFn: func(_ context.Context, _, _ string) (int64, error) {
						return 0, tt.repoErr
					},
				})
				_, err := svc.GetWalletStockQuantity(context.Background(), "w1", "AAPL")
				assert.ErrorIs(t, err, tt.repoErr)
			})
		}
	})
}

func TestService_GetStocks(t *testing.T) {
	t.Run("returns stocks from repo", func(t *testing.T) {
		expected := []models.Stock{
			{Name: "AAPL", Quantity: 100},
			{Name: "GOOG", Quantity: 50},
		}
		svc := service.NewMarket(&mockRepo{
			getBankStocksFn: func(_ context.Context) ([]models.Stock, error) {
				return expected, nil
			},
		})

		stocks, err := svc.GetStocks(context.Background())
		require.NoError(t, err)
		require.Len(t, stocks, 2)
		assert.Equal(t, "AAPL", stocks[0].Name)
		assert.Equal(t, "GOOG", stocks[1].Name)
	})

	t.Run("returns empty slice", func(t *testing.T) {
		svc := service.NewMarket(&mockRepo{
			getBankStocksFn: func(_ context.Context) ([]models.Stock, error) {
				return []models.Stock{}, nil
			},
		})

		stocks, err := svc.GetStocks(context.Background())
		require.NoError(t, err)
		assert.Empty(t, stocks)
	})
}

func TestService_SetStocks(t *testing.T) {
	t.Run("passes stocks to repo", func(t *testing.T) {
		input := models.SetStocks{
			Stocks: []models.Stock{
				{Name: "AAPL", Quantity: 100},
				{Name: "GOOG", Quantity: 50},
			},
		}
		var gotStocks []models.Stock
		svc := service.NewMarket(&mockRepo{
			setBankStocksFn: func(_ context.Context, stocks []models.Stock) error {
				gotStocks = stocks
				return nil
			},
		})

		require.NoError(t, svc.SetStocks(context.Background(), input))
		require.Len(t, gotStocks, 2)
		assert.Equal(t, "AAPL", gotStocks[0].Name)
		assert.Equal(t, int64(100), gotStocks[0].Quantity)
	})

	t.Run("propagates repo error", func(t *testing.T) {
		repoErr := errors.New("db write error")
		svc := service.NewMarket(&mockRepo{
			setBankStocksFn: func(_ context.Context, _ []models.Stock) error {
				return repoErr
			},
		})

		err := svc.SetStocks(context.Background(), models.SetStocks{})
		assert.ErrorIs(t, err, repoErr)
	})
}

func TestService_GetLog(t *testing.T) {
	t.Run("returns entries from repo", func(t *testing.T) {
		expected := []models.AuditLogEntry{
			{Type: "buy", WalletID: "w1", StockName: "AAPL"},
			{Type: "sell", WalletID: "w2", StockName: "GOOG"},
		}
		svc := service.NewMarket(&mockRepo{
			getAuditLogFn: func(_ context.Context) ([]models.AuditLogEntry, error) {
				return expected, nil
			},
		})

		log, err := svc.GetLog(context.Background())
		require.NoError(t, err)
		require.Len(t, log, 2)
		assert.Equal(t, "buy", log[0].Type)
		assert.Equal(t, "sell", log[1].Type)
	})

	t.Run("returns empty slice", func(t *testing.T) {
		svc := service.NewMarket(&mockRepo{
			getAuditLogFn: func(_ context.Context) ([]models.AuditLogEntry, error) {
				return []models.AuditLogEntry{}, nil
			},
		})

		log, err := svc.GetLog(context.Background())
		require.NoError(t, err)
		assert.Empty(t, log)
	})

	t.Run("propagates repo error", func(t *testing.T) {
		repoErr := errors.New("db read error")
		svc := service.NewMarket(&mockRepo{
			getAuditLogFn: func(_ context.Context) ([]models.AuditLogEntry, error) {
				return nil, repoErr
			},
		})

		_, err := svc.GetLog(context.Background())
		assert.ErrorIs(t, err, repoErr)
	})
}
