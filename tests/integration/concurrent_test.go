package integration_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"remitly_task/internal/models"
	"remitly_task/internal/repository"
)

func TestStore_Concurrent(t *testing.T) {
	t.Run("never oversells", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		const initial = 5
		const goroutines = 20
		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{
			{Name: "AAPL", Quantity: initial},
		}))

		var wg sync.WaitGroup
		var bought int64
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				err := store.ApplyWalletOperation(context.Background(), fmt.Sprintf("wallet-%d", i), "AAPL", "buy")
				if err == nil {
					atomic.AddInt64(&bought, 1)
				}
			}(i)
		}
		wg.Wait()

		assert.Equal(t, int64(initial), bought, "exactly initial stock units should be sold")

		stocks, err := store.GetBankStocks(context.Background())
		require.NoError(t, err)
		for _, s := range stocks {
			if s.Name == "AAPL" {
				assert.Equal(t, int64(0), s.Quantity, "bank must reach exactly zero")
			}
		}
	})

	t.Run("quantity is conserved", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		const initial int64 = 20
		const wallets = 10
		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{
			{Name: "AAPL", Quantity: initial},
		}))

		var wg sync.WaitGroup
		for i := 0; i < wallets; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				walletID := fmt.Sprintf("wallet-%d", i)
				for j := 0; j < 3; j++ {
					_ = store.ApplyWalletOperation(context.Background(), walletID, "AAPL", "buy")
				}
				_ = store.ApplyWalletOperation(context.Background(), walletID, "AAPL", "sell")
			}(i)
		}
		wg.Wait()

		bankStocks, err := store.GetBankStocks(context.Background())
		require.NoError(t, err)
		var total int64
		for _, s := range bankStocks {
			if s.Name == "AAPL" {
				total += s.Quantity
			}
		}
		for i := 0; i < wallets; i++ {
			// Ignore errors: ErrWalletNotFound / ErrStockNotFound both mean qty = 0.
			qty, _ := store.GetWalletStockQuantity(context.Background(), fmt.Sprintf("wallet-%d", i), "AAPL")
			total += qty
		}

		assert.Equal(t, initial, total, "total quantity across bank and all wallets must equal initial")
	})

	t.Run("sell never goes negative", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		const bankQty = 5
		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{
			{Name: "AAPL", Quantity: bankQty},
		}))
		for i := 0; i < bankQty; i++ {
			require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy"))
		}

		// 10 goroutines all try to sell from a wallet that holds only 5.
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "sell")
			}()
		}
		wg.Wait()

		qty, err := store.GetWalletStockQuantity(context.Background(), "wallet1", "AAPL")
		if err != nil {
			assert.ErrorIs(t, err, repository.ErrStockNotFound, "wallet exists but holds no AAPL after selling all")
		} else {
			assert.Equal(t, int64(0), qty, "wallet quantity must not go below zero")
		}
	})
}
