package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"remitly_task/internal/config"
	"remitly_task/internal/models"
	"remitly_task/internal/repository/postgres"
	"remitly_task/internal/repository"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func dbDSN() string {
	return fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		envOr("DB_HOST", "localhost"),
		envOr("DB_PORT", "5432"),
		envOr("DB_NAME", "stocks"),
		envOr("DB_USER", "stocks"),
		envOr("DB_PASSWORD", "stocks"),
	)
}

func newStore(t *testing.T) *postgres.Store {
	t.Helper()
	cfg := config.Config{
		DBHost:     envOr("DB_HOST", "localhost"),
		DBPort:     envOr("DB_PORT", "5432"),
		DBName:     envOr("DB_NAME", "stocks"),
		DBUser:     envOr("DB_USER", "stocks"),
		DBPassword: envOr("DB_PASSWORD", "stocks"),
	}
	store, err := postgres.NewStore(cfg)
	if err != nil {
		t.Skipf("skipping integration tests: cannot connect to database: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// clearDB truncates all tables between tests for isolation.
func clearDB(t *testing.T) {
	t.Helper()
	db, err := sql.Open("pgx", dbDSN())
	if err != nil {
		t.Skipf("skipping: cannot open db for cleanup: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skipf("skipping: cannot ping db: %v", err)
	}
	_, err = db.ExecContext(context.Background(),
		`TRUNCATE TABLE audit_log, wallet_stocks, wallets, bank_stocks RESTART IDENTITY CASCADE`)
	require.NoError(t, err, "clearDB")
}

func TestStore_BankStocks(t *testing.T) {
	t.Run("empty on fresh DB", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		stocks, err := store.GetBankStocks(context.Background())
		require.NoError(t, err)
		assert.Empty(t, stocks)
	})

	t.Run("set and get round-trips correctly", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{
			{Name: "AAPL", Quantity: 100},
			{Name: "GOOG", Quantity: 50},
		}))

		got, err := store.GetBankStocks(context.Background())
		require.NoError(t, err)
		require.Len(t, got, 2)
		// Stocks are returned alphabetically by name.
		assert.Equal(t, "AAPL", got[0].Name)
		assert.Equal(t, int64(100), got[0].Quantity)
		assert.Equal(t, "GOOG", got[1].Name)
		assert.Equal(t, int64(50), got[1].Quantity)
	})

	t.Run("zeroes unlisted stocks on subsequent set", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{
			{Name: "AAPL", Quantity: 100},
			{Name: "GOOG", Quantity: 50},
		}))
		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{
			{Name: "AAPL", Quantity: 200},
		}))

		got, err := store.GetBankStocks(context.Background())
		require.NoError(t, err)
		stockMap := make(map[string]int64)
		for _, s := range got {
			stockMap[s.Name] = s.Quantity
		}
		assert.Equal(t, int64(200), stockMap["AAPL"])
		assert.Equal(t, int64(0), stockMap["GOOG"], "GOOG should be zeroed, not deleted")
	})

	t.Run("idempotent on repeated set", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		stocks := []models.Stock{{Name: "AAPL", Quantity: 100}}
		for i := 0; i < 3; i++ {
			require.NoError(t, store.SetBankStocks(context.Background(), stocks), "iteration %d", i)
		}

		got, err := store.GetBankStocks(context.Background())
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, int64(100), got[0].Quantity)
	})

	t.Run("empty list zeroes all stocks", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 10}}))
		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{}))

		got, err := store.GetBankStocks(context.Background())
		require.NoError(t, err)
		for _, s := range got {
			if s.Name == "AAPL" {
				assert.Equal(t, int64(0), s.Quantity)
			}
		}
	})

	t.Run("zeroed stocks return 400 not 404, wallets unaffected", func(t *testing.T) {
		//   Scenario:
		//   1. wallet buys stock1
		//   2. POST /stocks with only stock2 — stock1 is zeroed, not deleted
		//   3. buying stock1 must return ErrNoBankStock (400), not ErrStockNotFound (404)
		//   4. buying stock2 (introduced with qty 0) must also return ErrNoBankStock (400)
		//   5. wallet can still sell stock1 (wallets are untouched by SetBankStocks)
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "stock1", Quantity: 5}}))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "stock1", "buy"))

		// POST /stocks with only stock2; stock1 is not listed — it gets zeroed, not deleted.
		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "stock2", Quantity: 0}}))

		err := store.ApplyWalletOperation(context.Background(), "wallet2", "stock1", "buy")
		assert.ErrorIs(t, err, repository.ErrNoBankStock, "stock1 zeroed by SetBankStocks should give 400 not 404")

		err = store.ApplyWalletOperation(context.Background(), "wallet2", "stock2", "buy")
		assert.ErrorIs(t, err, repository.ErrNoBankStock, "stock2 introduced with qty=0 should give 400 not 404")

		// Wallet still holds stock1; selling it back to the bank must succeed.
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "stock1", "sell"))
	})
}

func TestStore_Buy(t *testing.T) {
	t.Run("decreases bank and increases wallet", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 10}}))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy"))

		bankStocks, err := store.GetBankStocks(context.Background())
		require.NoError(t, err)
		bankMap := make(map[string]int64)
		for _, s := range bankStocks {
			bankMap[s.Name] = s.Quantity
		}
		assert.Equal(t, int64(9), bankMap["AAPL"])

		qty, err := store.GetWalletStockQuantity(context.Background(), "wallet1", "AAPL")
		require.NoError(t, err)
		assert.Equal(t, int64(1), qty)
	})

	t.Run("creates wallet if not exists", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 5}}))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "brand-new-wallet", "AAPL", "buy"))

		wallet, err := store.GetWallet(context.Background(), "brand-new-wallet")
		require.NoError(t, err)
		assert.Equal(t, "brand-new-wallet", wallet.ID)
	})

	t.Run("accumulates on multiple buys", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 5}}))
		for i := 0; i < 3; i++ {
			require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy"), "iteration %d", i)
		}

		qty, err := store.GetWalletStockQuantity(context.Background(), "wallet1", "AAPL")
		require.NoError(t, err)
		assert.Equal(t, int64(3), qty)
	})

	t.Run("failed buy does not create wallet", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 0}}))
		_ = store.ApplyWalletOperation(context.Background(), "ghost-wallet", "AAPL", "buy")

		_, err := store.GetWallet(context.Background(), "ghost-wallet")
		assert.ErrorIs(t, err, repository.ErrWalletNotFound)
	})

	t.Run("returns error", func(t *testing.T) {
		tests := []struct {
			name    string
			setup   func(*testing.T, *postgres.Store)
			stock   string
			wantErr error
		}{
			{
				name:    "stock not found",
				setup:   func(_ *testing.T, _ *postgres.Store) {},
				stock:   "NONEXISTENT",
				wantErr: repository.ErrStockNotFound,
			},
			{
				name: "no bank stock",
				setup: func(t *testing.T, store *postgres.Store) {
					require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 0}}))
				},
				stock:   "AAPL",
				wantErr: repository.ErrNoBankStock,
			},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				store := newStore(t)
				clearDB(t)
				tt.setup(t, store)
				err := store.ApplyWalletOperation(context.Background(), "wallet1", tt.stock, "buy")
				assert.ErrorIs(t, err, tt.wantErr)
			})
		}
	})
}

func TestStore_Sell(t *testing.T) {
	t.Run("decreases wallet and increases bank", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 5}}))
		for i := 0; i < 2; i++ {
			require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy"), "buy %d", i)
		}
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "sell"))

		bankStocks, err := store.GetBankStocks(context.Background())
		require.NoError(t, err)
		bankMap := make(map[string]int64)
		for _, s := range bankStocks {
			bankMap[s.Name] = s.Quantity
		}
		assert.Equal(t, int64(4), bankMap["AAPL"])

		qty, err := store.GetWalletStockQuantity(context.Background(), "wallet1", "AAPL")
		require.NoError(t, err)
		assert.Equal(t, int64(1), qty)
	})

	t.Run("removes stock row when quantity reaches zero", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 5}}))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy"))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "sell"))

		wallet, err := store.GetWallet(context.Background(), "wallet1")
		require.NoError(t, err)
		assert.Empty(t, wallet.Stocks)
	})

	t.Run("returns error", func(t *testing.T) {
		tests := []struct {
			name    string
			setup   func(*testing.T, *postgres.Store)
			stock   string
			wantErr error
		}{
			{
				name: "no wallet stock",
				setup: func(t *testing.T, store *postgres.Store) {
					require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 5}}))
					require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy"))
					require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "sell"))
				},
				stock:   "AAPL",
				wantErr: repository.ErrNoWalletStock,
			},
			{
				name: "stock not found in wallet",
				setup: func(t *testing.T, store *postgres.Store) {
					require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 1}}))
					require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy"))
				},
				stock:   "NONEXISTENT",
				wantErr: repository.ErrStockNotFound,
			},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				store := newStore(t)
				clearDB(t)
				tt.setup(t, store)
				err := store.ApplyWalletOperation(context.Background(), "wallet1", tt.stock, "sell")
				assert.ErrorIs(t, err, tt.wantErr)
			})
		}
	})
}

func TestStore_GetWallet(t *testing.T) {
	t.Run("not found returns error", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		_, err := store.GetWallet(context.Background(), "nonexistent")
		assert.ErrorIs(t, err, repository.ErrWalletNotFound)
	})

	t.Run("returns wallet with multiple stocks", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{
			{Name: "AAPL", Quantity: 10},
			{Name: "GOOG", Quantity: 10},
		}))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy"))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "GOOG", "buy"))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "GOOG", "buy"))

		wallet, err := store.GetWallet(context.Background(), "wallet1")
		require.NoError(t, err)
		assert.Equal(t, "wallet1", wallet.ID)
		require.Len(t, wallet.Stocks, 2)
		// Stocks are returned alphabetically by name.
		assert.Equal(t, "AAPL", wallet.Stocks[0].Name)
		assert.Equal(t, int64(1), wallet.Stocks[0].Quantity)
		assert.Equal(t, "GOOG", wallet.Stocks[1].Name)
		assert.Equal(t, int64(2), wallet.Stocks[1].Quantity)
	})

	t.Run("empty after selling all stocks", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 5}}))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy"))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "sell"))

		wallet, err := store.GetWallet(context.Background(), "wallet1")
		require.NoError(t, err)
		assert.Empty(t, wallet.Stocks)
	})
}

func TestStore_GetWalletStockQuantity(t *testing.T) {
	t.Run("after multiple buys", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 10}}))
		for i := 0; i < 5; i++ {
			require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy"), "buy %d", i)
		}

		qty, err := store.GetWalletStockQuantity(context.Background(), "wallet1", "AAPL")
		require.NoError(t, err)
		assert.Equal(t, int64(5), qty)
	})

	t.Run("zero when stock not held", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{
			{Name: "AAPL", Quantity: 5},
			{Name: "GOOG", Quantity: 5},
		}))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy"))

		qty, err := store.GetWalletStockQuantity(context.Background(), "wallet1", "GOOG")
		require.NoError(t, err)
		assert.Equal(t, int64(0), qty)
	})

	t.Run("returns error", func(t *testing.T) {
		tests := []struct {
			name     string
			setup    func(*testing.T, *postgres.Store)
			walletID string
			stock    string
			wantErr  error
		}{
			{
				name: "wallet not found",
				setup: func(t *testing.T, store *postgres.Store) {
					require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 10}}))
				},
				walletID: "nonexistent",
				stock:    "AAPL",
				wantErr:  repository.ErrWalletNotFound,
			},
			{
				name: "stock not found in wallet",
				setup: func(t *testing.T, store *postgres.Store) {
					require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 1}}))
					require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy"))
				},
				walletID: "wallet1",
				stock:    "NONEXISTENT",
				wantErr:  repository.ErrStockNotFound,
			},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				store := newStore(t)
				clearDB(t)
				tt.setup(t, store)
				_, err := store.GetWalletStockQuantity(context.Background(), tt.walletID, tt.stock)
				assert.ErrorIs(t, err, tt.wantErr)
			})
		}
	})
}

func TestStore_GetAuditLog(t *testing.T) {
	t.Run("empty on fresh DB", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		entries, err := store.GetAuditLog(context.Background())
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("records successful buy and sell", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 10}}))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy"))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "sell"))

		entries, err := store.GetAuditLog(context.Background())
		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, "buy", entries[0].Type)
		assert.Equal(t, "wallet1", entries[0].WalletID)
		assert.Equal(t, "AAPL", entries[0].StockName)
		assert.Equal(t, "sell", entries[1].Type)
		assert.Equal(t, "wallet1", entries[1].WalletID)
		assert.Equal(t, "AAPL", entries[1].StockName)
	})

	t.Run("excludes failed operations", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 0}}))
		_ = store.ApplyWalletOperation(context.Background(), "wallet1", "AAPL", "buy")

		entries, err := store.GetAuditLog(context.Background())
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("ordered by occurrence", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{
			{Name: "AAPL", Quantity: 5},
			{Name: "GOOG", Quantity: 5},
		}))

		ops := []struct{ stock, opType string }{
			{"AAPL", "buy"},
			{"GOOG", "buy"},
			{"AAPL", "sell"},
		}
		for _, op := range ops {
			require.NoError(t, store.ApplyWalletOperation(context.Background(), "wallet1", op.stock, op.opType), "op %+v", op)
		}

		entries, err := store.GetAuditLog(context.Background())
		require.NoError(t, err)
		require.Len(t, entries, 3)

		expected := []struct{ op, stock string }{
			{"buy", "AAPL"}, {"buy", "GOOG"}, {"sell", "AAPL"},
		}
		for i, e := range expected {
			assert.Equal(t, e.op, entries[i].Type, "entry[%d].Type", i)
			assert.Equal(t, e.stock, entries[i].StockName, "entry[%d].StockName", i)
		}
	})

	t.Run("multiple wallets", func(t *testing.T) {
		store := newStore(t)
		clearDB(t)

		require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{{Name: "AAPL", Quantity: 10}}))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "walletA", "AAPL", "buy"))
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "walletB", "AAPL", "buy"))

		entries, err := store.GetAuditLog(context.Background())
		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, "walletA", entries[0].WalletID)
		assert.Equal(t, "walletB", entries[1].WalletID)
	})
}

func TestStore_FullWorkflow_BuySellMultipleStocks(t *testing.T) {
	store := newStore(t)
	clearDB(t)

	require.NoError(t, store.SetBankStocks(context.Background(), []models.Stock{
		{Name: "AAPL", Quantity: 5},
		{Name: "TSLA", Quantity: 3},
	}))

	// Wallet A buys 2 AAPL and 1 TSLA.
	for i := 0; i < 2; i++ {
		require.NoError(t, store.ApplyWalletOperation(context.Background(), "walletA", "AAPL", "buy"), "walletA buy AAPL %d", i)
	}
	require.NoError(t, store.ApplyWalletOperation(context.Background(), "walletA", "TSLA", "buy"))

	// Wallet B buys 1 AAPL.
	require.NoError(t, store.ApplyWalletOperation(context.Background(), "walletB", "AAPL", "buy"))

	// Wallet A sells 1 AAPL back.
	require.NoError(t, store.ApplyWalletOperation(context.Background(), "walletA", "AAPL", "sell"))

	// Verify bank state: AAPL started 5, bought 3, sold 1 → 3; TSLA started 3, bought 1 → 2.
	bankStocks, err := store.GetBankStocks(context.Background())
	require.NoError(t, err)
	bankMap := make(map[string]int64)
	for _, s := range bankStocks {
		bankMap[s.Name] = s.Quantity
	}
	assert.Equal(t, int64(3), bankMap["AAPL"])
	assert.Equal(t, int64(2), bankMap["TSLA"])

	// Verify wallet A: 1 AAPL (bought 2, sold 1), 1 TSLA.
	walletA, err := store.GetWallet(context.Background(), "walletA")
	require.NoError(t, err)
	walletAMap := make(map[string]int64)
	for _, s := range walletA.Stocks {
		walletAMap[s.Name] = s.Quantity
	}
	assert.Equal(t, int64(1), walletAMap["AAPL"])
	assert.Equal(t, int64(1), walletAMap["TSLA"])

	// Audit log should have 5 entries: 2 buys AAPL + 1 buy TSLA + 1 buy AAPL + 1 sell AAPL.
	log, err := store.GetAuditLog(context.Background())
	require.NoError(t, err)
	assert.Len(t, log, 5)
}
