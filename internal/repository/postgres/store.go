package postgres

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"

	"remitly_task/internal/config"
	"remitly_task/internal/models"
	"remitly_task/internal/repository"
)

type Store struct {
	db *sql.DB
}

func NewStore(cfg config.Config) (*Store, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
		cfg.DBUser,
		cfg.DBPassword,
	)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) ApplyWalletOperation(ctx context.Context, walletID string, stockName string, operation string) (err error) {
	op := models.WalletStockOperation{
		WalletID:  walletID,
		StockName: stockName,
		Type:      operation,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var bankStock models.StockBalance
	bankStock.Name = op.StockName
	err = tx.QueryRowContext(ctx, `
		SELECT quantity
		FROM bank_stocks
		WHERE stock_name = $1
		FOR UPDATE
	`, op.StockName).Scan(&bankStock.Quantity)
	if err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrStockNotFound
		}
		return err
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO wallets (id) VALUES ($1)
		ON CONFLICT (id) DO NOTHING
	`, op.WalletID); err != nil {
		return err
	}

	switch op.Type {
	case "buy":
		if bankStock.Quantity <= 0 {
			return repository.ErrNoBankStock
		}

		if _, err = tx.ExecContext(ctx, `
			UPDATE bank_stocks
			SET quantity = quantity - 1, updated_at = NOW()
			WHERE stock_name = $1
		`, op.StockName); err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, `
			INSERT INTO wallet_stocks (wallet_id, stock_name, quantity)
			VALUES ($1, $2, 1)
			ON CONFLICT (wallet_id, stock_name)
			DO UPDATE SET quantity = wallet_stocks.quantity + 1, updated_at = NOW()
		`, op.WalletID, op.StockName); err != nil {
			return err
		}
	case "sell":
		var walletStock models.StockBalance
		walletStock.Name = op.StockName
		err = tx.QueryRowContext(ctx, `
			SELECT quantity
			FROM wallet_stocks
			WHERE wallet_id = $1 AND stock_name = $2
			FOR UPDATE
		`, op.WalletID, op.StockName).Scan(&walletStock.Quantity)
		if err != nil {
			if err == sql.ErrNoRows {
				return repository.ErrNoWalletStock
			}
			return err
		}
		if walletStock.Quantity <= 0 {
			return repository.ErrNoWalletStock
		}

		if _, err = tx.ExecContext(ctx, `
			UPDATE wallet_stocks
			SET quantity = quantity - 1, updated_at = NOW()
			WHERE wallet_id = $1 AND stock_name = $2
		`, op.WalletID, op.StockName); err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, `
			DELETE FROM wallet_stocks
			WHERE wallet_id = $1 AND stock_name = $2 AND quantity = 0
		`, op.WalletID, op.StockName); err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, `
			UPDATE bank_stocks
			SET quantity = quantity + 1, updated_at = NOW()
			WHERE stock_name = $1
		`, op.StockName); err != nil {
			return err
		}
	default:
		return repository.ErrInvalidOperation
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_log (operation_type, wallet_id, stock_name)
		VALUES ($1, $2, $3)
	`, op.Type, op.WalletID, op.StockName); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *Store) GetWallet(ctx context.Context, walletID string) (models.Wallet, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM wallets WHERE id = $1)
	`, walletID).Scan(&exists)
	if err != nil {
		return models.Wallet{}, err
	}
	if !exists {
		return models.Wallet{}, repository.ErrWalletNotFound
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT stock_name, quantity
		FROM wallet_stocks
		WHERE wallet_id = $1
		ORDER BY stock_name ASC
	`, walletID)
	if err != nil {
		return models.Wallet{}, err
	}
	defer rows.Close()

	stocks := make([]models.Stock, 0)
	for rows.Next() {
		var stockRow models.StockBalance
		if err := rows.Scan(&stockRow.Name, &stockRow.Quantity); err != nil {
			return models.Wallet{}, err
		}
		stocks = append(stocks, models.Stock{Name: stockRow.Name, Quantity: stockRow.Quantity})
	}
	if err := rows.Err(); err != nil {
		return models.Wallet{}, err
	}

	return models.Wallet{
		ID:     walletID,
		Stocks: stocks,
	}, nil
}

func (s *Store) GetWalletStockQuantity(ctx context.Context, walletID string, stockName string) (int64, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM wallets WHERE id = $1)
	`, walletID).Scan(&exists)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, repository.ErrWalletNotFound
	}

	err = s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM bank_stocks WHERE stock_name = $1)
	`, stockName).Scan(&exists)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, repository.ErrStockNotFound
	}

	var quantity int64
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(quantity, 0)
		FROM wallet_stocks
		WHERE wallet_id = $1 AND stock_name = $2
	`, walletID, stockName).Scan(&quantity)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}

	return quantity, nil
}

func (s *Store) GetBankStocks(ctx context.Context) ([]models.Stock, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT stock_name, quantity
		FROM bank_stocks
		ORDER BY stock_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stocks := make([]models.Stock, 0)
	for rows.Next() {
		var stockRow models.StockBalance
		if err := rows.Scan(&stockRow.Name, &stockRow.Quantity); err != nil {
			return nil, err
		}
		stocks = append(stocks, models.Stock{Name: stockRow.Name, Quantity: stockRow.Quantity})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stocks, nil
}

func (s *Store) SetBankStocks(ctx context.Context, stocks []models.Stock) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `UPDATE bank_stocks SET quantity = 0, updated_at = NOW()`); err != nil {
		return err
	}

	for _, stock := range stocks {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO bank_stocks (stock_name, quantity)
			VALUES ($1, $2)
			ON CONFLICT (stock_name)
			DO UPDATE SET quantity = EXCLUDED.quantity, updated_at = NOW()
		`, stock.Name, stock.Quantity); err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *Store) GetAuditLog(ctx context.Context) ([]models.AuditLogEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT operation_type, wallet_id, stock_name
		FROM audit_log
		ORDER BY id ASC
		LIMIT 10000
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logEntries := make([]models.AuditLogEntry, 0)
	for rows.Next() {
		var logRow models.AuditLogEntry
		if err := rows.Scan(&logRow.Type, &logRow.WalletID, &logRow.StockName); err != nil {
			return nil, err
		}
		logEntries = append(logEntries, models.AuditLogEntry{
			Type:      logRow.Type,
			WalletID:  logRow.WalletID,
			StockName: logRow.StockName,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return logEntries, nil
}
