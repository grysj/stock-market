package unit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"remitly_task/internal/http/handlers"
	"remitly_task/internal/http/router"
	"remitly_task/internal/models"
	"remitly_task/internal/repository"
)

// mockService implements handlers.MarketService for handler tests.
type mockService struct {
	walletStockOperationFn   func(ctx context.Context, input models.WalletStockOperation) error
	getWalletFn              func(ctx context.Context, walletID string) (models.Wallet, error)
	getWalletStockQuantityFn func(ctx context.Context, walletID, stockName string) (int64, error)
	getStocksFn              func(ctx context.Context) ([]models.Stock, error)
	setStocksFn              func(ctx context.Context, input models.SetStocks) error
	getLogFn                 func(ctx context.Context) ([]models.AuditLogEntry, error)
}

func (m *mockService) WalletStockOperation(ctx context.Context, input models.WalletStockOperation) error {
	if m.walletStockOperationFn != nil {
		return m.walletStockOperationFn(ctx, input)
	}
	return nil
}

func (m *mockService) GetWallet(ctx context.Context, walletID string) (models.Wallet, error) {
	if m.getWalletFn != nil {
		return m.getWalletFn(ctx, walletID)
	}
	return models.Wallet{ID: walletID, Stocks: []models.Stock{}}, nil
}

func (m *mockService) GetWalletStockQuantity(ctx context.Context, walletID, stockName string) (int64, error) {
	if m.getWalletStockQuantityFn != nil {
		return m.getWalletStockQuantityFn(ctx, walletID, stockName)
	}
	return 0, nil
}

func (m *mockService) GetStocks(ctx context.Context) ([]models.Stock, error) {
	if m.getStocksFn != nil {
		return m.getStocksFn(ctx)
	}
	return []models.Stock{}, nil
}

func (m *mockService) SetStocks(ctx context.Context, input models.SetStocks) error {
	if m.setStocksFn != nil {
		return m.setStocksFn(ctx, input)
	}
	return nil
}

func (m *mockService) GetLog(ctx context.Context) ([]models.AuditLogEntry, error) {
	if m.getLogFn != nil {
		return m.getLogFn(ctx)
	}
	return []models.AuditLogEntry{}, nil
}

func newTestRouter(svc *mockService) http.Handler {
	h := handlers.New(svc)
	return router.New(h)
}

func TestHandler_WalletStockOperation(t *testing.T) {
	t.Run("valid operations return 200", func(t *testing.T) {
		tests := []struct {
			name string
			body string
		}{
			{"buy", `{"type":"buy"}`},
			{"sell", `{"type":"sell"}`},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				r := newTestRouter(&mockService{})
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL",
					bytes.NewBufferString(tt.body))
				r.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusOK, rec.Code)
			})
		}
	})

	t.Run("invalid request body returns 400", func(t *testing.T) {
		tests := []struct {
			name string
			body string
		}{
			{"malformed JSON", `not json`},
			{"unknown field", `{"type":"buy","unknown":"field"}`},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				r := newTestRouter(&mockService{})
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL",
					bytes.NewBufferString(tt.body))
				r.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			})
		}
	})

	t.Run("maps service errors to HTTP status codes", func(t *testing.T) {
		tests := []struct {
			name       string
			svcErr     error
			wantStatus int
		}{
			{"ErrInvalidOperation", repository.ErrInvalidOperation, http.StatusBadRequest},
			{"ErrStockNotFound", repository.ErrStockNotFound, http.StatusNotFound},
			{"ErrNoBankStock", repository.ErrNoBankStock, http.StatusBadRequest},
			{"ErrNoWalletStock", repository.ErrNoWalletStock, http.StatusBadRequest},
			{"unexpected db error", errors.New("db error"), http.StatusInternalServerError},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				svc := &mockService{
					walletStockOperationFn: func(_ context.Context, _ models.WalletStockOperation) error {
						return tt.svcErr
					},
				}
				r := newTestRouter(svc)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL",
					bytes.NewBufferString(`{"type":"buy"}`))
				r.ServeHTTP(rec, req)
				assert.Equal(t, tt.wantStatus, rec.Code)
			})
		}
	})

	t.Run("passes correct args to service", func(t *testing.T) {
		var gotOp models.WalletStockOperation
		svc := &mockService{
			walletStockOperationFn: func(_ context.Context, op models.WalletStockOperation) error {
				gotOp = op
				return nil
			},
		}
		r := newTestRouter(svc)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/wallets/wallet123/stocks/TSLA",
			bytes.NewBufferString(`{"type":"sell"}`))

		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "wallet123", gotOp.WalletID)
		assert.Equal(t, "TSLA", gotOp.StockName)
		assert.Equal(t, "sell", gotOp.Type)
	})
}

func TestHandler_GetWallet(t *testing.T) {
	t.Run("returns wallet with stocks", func(t *testing.T) {
		svc := &mockService{
			getWalletFn: func(_ context.Context, walletID string) (models.Wallet, error) {
				return models.Wallet{
					ID:     walletID,
					Stocks: []models.Stock{{Name: "AAPL", Quantity: 3}},
				}, nil
			},
		}
		r := newTestRouter(svc)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/wallets/w1", nil)

		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp models.Wallet
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.Equal(t, "w1", resp.ID)
		require.Len(t, resp.Stocks, 1)
		assert.Equal(t, "AAPL", resp.Stocks[0].Name)
		assert.Equal(t, int64(3), resp.Stocks[0].Quantity)
	})

	t.Run("returns 200 for empty wallet", func(t *testing.T) {
		svc := &mockService{
			getWalletFn: func(_ context.Context, walletID string) (models.Wallet, error) {
				return models.Wallet{ID: walletID, Stocks: []models.Stock{}}, nil
			},
		}
		r := newTestRouter(svc)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/wallets/w1", nil)

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("maps service errors to HTTP status codes", func(t *testing.T) {
		tests := []struct {
			name       string
			svcErr     error
			wantStatus int
		}{
			{"ErrWalletNotFound", repository.ErrWalletNotFound, http.StatusNotFound},
			{"unexpected db error", errors.New("db error"), http.StatusInternalServerError},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				svc := &mockService{
					getWalletFn: func(_ context.Context, _ string) (models.Wallet, error) {
						return models.Wallet{}, tt.svcErr
					},
				}
				r := newTestRouter(svc)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/wallets/w1", nil)
				r.ServeHTTP(rec, req)
				assert.Equal(t, tt.wantStatus, rec.Code)
			})
		}
	})
}

func TestHandler_GetWalletStock(t *testing.T) {
	t.Run("returns plain text quantity", func(t *testing.T) {
		tests := []struct {
			name     string
			qty      int64
			wantBody string
		}{
			{"non-zero", 99, "99"},
			{"zero", 0, "0"},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				svc := &mockService{
					getWalletStockQuantityFn: func(_ context.Context, _, _ string) (int64, error) {
						return tt.qty, nil
					},
				}
				r := newTestRouter(svc)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/wallets/w1/stocks/AAPL", nil)
				r.ServeHTTP(rec, req)
				require.Equal(t, http.StatusOK, rec.Code)
				body, err := io.ReadAll(rec.Body)
				require.NoError(t, err)
				assert.Equal(t, tt.wantBody, string(body))
			})
		}
	})

	t.Run("maps service errors to HTTP status codes", func(t *testing.T) {
		tests := []struct {
			name       string
			svcErr     error
			wantStatus int
		}{
			{"ErrWalletNotFound", repository.ErrWalletNotFound, http.StatusNotFound},
			{"ErrStockNotFound", repository.ErrStockNotFound, http.StatusNotFound},
			{"unexpected db error", errors.New("db error"), http.StatusInternalServerError},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				svc := &mockService{
					getWalletStockQuantityFn: func(_ context.Context, _, _ string) (int64, error) {
						return 0, tt.svcErr
					},
				}
				r := newTestRouter(svc)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/wallets/w1/stocks/AAPL", nil)
				r.ServeHTTP(rec, req)
				assert.Equal(t, tt.wantStatus, rec.Code)
			})
		}
	})
}

func TestHandler_GetStocks(t *testing.T) {
	t.Run("returns stocks", func(t *testing.T) {
		svc := &mockService{
			getStocksFn: func(_ context.Context) ([]models.Stock, error) {
				return []models.Stock{{Name: "AAPL", Quantity: 100}}, nil
			},
		}
		r := newTestRouter(svc)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/stocks", nil)

		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Stocks []models.Stock `json:"stocks"`
		}
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		require.Len(t, resp.Stocks, 1)
		assert.Equal(t, "AAPL", resp.Stocks[0].Name)
		assert.Equal(t, int64(100), resp.Stocks[0].Quantity)
	})

	t.Run("returns 200 for empty inventory", func(t *testing.T) {
		r := newTestRouter(&mockService{})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/stocks", nil)

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("propagates service error as 500", func(t *testing.T) {
		svc := &mockService{
			getStocksFn: func(_ context.Context) ([]models.Stock, error) {
				return nil, errors.New("db error")
			},
		}
		r := newTestRouter(svc)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/stocks", nil)

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandler_SetStocks(t *testing.T) {
	t.Run("returns 200 for valid inputs", func(t *testing.T) {
		tests := []struct {
			name string
			body string
		}{
			{"with stocks", `{"stocks":[{"name":"AAPL","quantity":100}]}`},
			{"empty list", `{"stocks":[]}`},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				r := newTestRouter(&mockService{})
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/stocks",
					bytes.NewBufferString(tt.body))
				r.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusOK, rec.Code)
			})
		}
	})

	t.Run("returns 400 for invalid inputs", func(t *testing.T) {
		tests := []struct {
			name string
			body string
		}{
			{"malformed JSON", `{bad json}`},
			{"empty stock name", `{"stocks":[{"name":"","quantity":100}]}`},
			{"negative quantity", `{"stocks":[{"name":"AAPL","quantity":-1}]}`},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				r := newTestRouter(&mockService{})
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/stocks",
					bytes.NewBufferString(tt.body))
				r.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusBadRequest, rec.Code)
			})
		}
	})

	t.Run("passes correct data to service", func(t *testing.T) {
		var gotInput models.SetStocks
		svc := &mockService{
			setStocksFn: func(_ context.Context, input models.SetStocks) error {
				gotInput = input
				return nil
			},
		}
		r := newTestRouter(svc)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/stocks",
			bytes.NewBufferString(`{"stocks":[{"name":"AAPL","quantity":50},{"name":"GOOG","quantity":25}]}`))

		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Len(t, gotInput.Stocks, 2)
		assert.Equal(t, "AAPL", gotInput.Stocks[0].Name)
		assert.Equal(t, int64(50), gotInput.Stocks[0].Quantity)
		assert.Equal(t, "GOOG", gotInput.Stocks[1].Name)
		assert.Equal(t, int64(25), gotInput.Stocks[1].Quantity)
	})

	t.Run("propagates service error as 500", func(t *testing.T) {
		svc := &mockService{
			setStocksFn: func(_ context.Context, _ models.SetStocks) error {
				return errors.New("db error")
			},
		}
		r := newTestRouter(svc)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/stocks",
			bytes.NewBufferString(`{"stocks":[{"name":"AAPL","quantity":100}]}`))

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandler_GetLog(t *testing.T) {
	t.Run("returns log entries", func(t *testing.T) {
		svc := &mockService{
			getLogFn: func(_ context.Context) ([]models.AuditLogEntry, error) {
				return []models.AuditLogEntry{
					{Type: "buy", WalletID: "w1", StockName: "AAPL"},
					{Type: "sell", WalletID: "w1", StockName: "AAPL"},
				}, nil
			},
		}
		r := newTestRouter(svc)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/log", nil)

		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Log []models.AuditLogEntry `json:"log"`
		}
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		require.Len(t, resp.Log, 2)
		assert.Equal(t, "buy", resp.Log[0].Type)
		assert.Equal(t, "sell", resp.Log[1].Type)
	})

	t.Run("returns 200 for empty log", func(t *testing.T) {
		r := newTestRouter(&mockService{})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/log", nil)

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("propagates service error as 500", func(t *testing.T) {
		svc := &mockService{
			getLogFn: func(_ context.Context) ([]models.AuditLogEntry, error) {
				return nil, errors.New("db error")
			},
		}
		r := newTestRouter(svc)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/log", nil)

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandler_Health(t *testing.T) {
	r := newTestRouter(&mockService{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "ok", resp.Status)
}

func TestHandler_ErrorResponse_HasErrorField(t *testing.T) {
	svc := &mockService{
		getWalletFn: func(_ context.Context, _ string) (models.Wallet, error) {
			return models.Wallet{}, repository.ErrWalletNotFound
		},
	}
	r := newTestRouter(svc)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wallets/missing", nil)

	r.ServeHTTP(rec, req)

	var resp struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.NotEmpty(t, resp.Error)
}

func TestHandler_ContentType(t *testing.T) {
	tests := []struct {
		name        string
		svc         *mockService
		method      string
		path        string
		wantCT      string
	}{
		{
			name:   "JSON endpoint",
			svc:    &mockService{},
			method: http.MethodGet,
			path:   "/stocks",
			wantCT: "application/json",
		},
		{
			name: "plain text quantity endpoint",
			svc: &mockService{
				getWalletStockQuantityFn: func(_ context.Context, _, _ string) (int64, error) {
					return 5, nil
				},
			},
			method: http.MethodGet,
			path:   "/wallets/w1/stocks/AAPL",
			wantCT: "text/plain; charset=utf-8",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRouter(tt.svc)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			r.ServeHTTP(rec, req)
			assert.Equal(t, tt.wantCT, rec.Header().Get("Content-Type"))
		})
	}
}
