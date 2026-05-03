package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"remitly_task/internal/models"
)

func baseURL() string {
	if url := os.Getenv("BASE_URL"); url != "" {
		return strings.TrimRight(url, "/")
	}
	return "http://localhost:8080"
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// setStocks sets the bank inventory, failing the test on any error.
func setStocks(t *testing.T, stocks []models.Stock) {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{"stocks": stocks})
	resp, err := http.Post(baseURL()+"/stocks", "application/json", bytes.NewReader(body))
	require.NoError(t, err, "POST /stocks")
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST /stocks response: %s", b)
}

// buyStock sends a buy request and returns the response status.
func buyStock(t *testing.T, walletID, stockName string) int {
	t.Helper()
	resp, err := http.Post(
		fmt.Sprintf("%s/wallets/%s/stocks/%s", baseURL(), walletID, stockName),
		"application/json",
		bytes.NewBufferString(`{"type":"buy"}`),
	)
	require.NoError(t, err, "buy request")
	defer resp.Body.Close()
	return resp.StatusCode
}

// sellStock sends a sell request and returns the response status.
func sellStock(t *testing.T, walletID, stockName string) int {
	t.Helper()
	resp, err := http.Post(
		fmt.Sprintf("%s/wallets/%s/stocks/%s", baseURL(), walletID, stockName),
		"application/json",
		bytes.NewBufferString(`{"type":"sell"}`),
	)
	require.NoError(t, err, "sell request")
	defer resp.Body.Close()
	return resp.StatusCode
}

// uniqueWalletID creates a wallet ID that is unique per test run to avoid state conflicts.
func uniqueWalletID(t *testing.T) string {
	return fmt.Sprintf("e2e-%s-%d", strings.NewReplacer("/", "-", " ", "-").Replace(t.Name()), time.Now().UnixNano())
}

// -------------------------------------------------------
// Health
// -------------------------------------------------------

func TestE2E_Health(t *testing.T) {
	resp, err := http.Get(baseURL() + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var result struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "ok", result.Status)
}

// -------------------------------------------------------
// Bank stocks
// -------------------------------------------------------

func TestE2E_SetStocks(t *testing.T) {
	t.Run("set and get round-trips correctly", func(t *testing.T) {
		setStocks(t, []models.Stock{
			{Name: "AAPL", Quantity: 100},
			{Name: "GOOG", Quantity: 50},
		})

		resp, err := http.Get(baseURL() + "/stocks")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		var result struct {
			Stocks []models.Stock `json:"stocks"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		stockMap := make(map[string]int64)
		for _, s := range result.Stocks {
			stockMap[s.Name] = s.Quantity
		}
		assert.Equal(t, int64(100), stockMap["AAPL"])
		assert.Equal(t, int64(50), stockMap["GOOG"])
	})

	t.Run("invalid inputs return 400", func(t *testing.T) {
		tests := []struct {
			name string
			body string
		}{
			{"malformed JSON", `not json`},
			{"negative quantity", `{"stocks":[{"name":"AAPL","quantity":-1}]}`},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				resp, err := http.Post(baseURL()+"/stocks", "application/json",
					bytes.NewBufferString(tt.body))
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			})
		}
	})
}

// -------------------------------------------------------
// Buy
// -------------------------------------------------------

func TestE2E_Buy(t *testing.T) {
	t.Run("returns 200 and creates wallet with stock", func(t *testing.T) {
		setStocks(t, []models.Stock{{Name: "AAPL", Quantity: 10}})
		wid := uniqueWalletID(t)

		require.Equal(t, http.StatusOK, buyStock(t, wid, "AAPL"))

		resp, err := http.Get(fmt.Sprintf("%s/wallets/%s", baseURL(), wid))
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		var wallet models.Wallet
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&wallet))
		require.Len(t, wallet.Stocks, 1)
		assert.Equal(t, "AAPL", wallet.Stocks[0].Name)
		assert.Equal(t, int64(1), wallet.Stocks[0].Quantity)
	})

	t.Run("decreases bank balance", func(t *testing.T) {
		setStocks(t, []models.Stock{{Name: "AAPL", Quantity: 10}})
		wid := uniqueWalletID(t)

		buyStock(t, wid, "AAPL")
		buyStock(t, wid, "AAPL")

		resp, err := http.Get(baseURL() + "/stocks")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result struct {
			Stocks []models.Stock `json:"stocks"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		stockMap := make(map[string]int64)
		for _, s := range result.Stocks {
			stockMap[s.Name] = s.Quantity
		}
		assert.Equal(t, int64(8), stockMap["AAPL"], "expected bank AAPL=8 after 2 buys")
	})

	t.Run("invalid request body returns 400", func(t *testing.T) {
		setStocks(t, []models.Stock{{Name: "AAPL", Quantity: 5}})
		wid := uniqueWalletID(t)

		tests := []struct {
			name string
			body string
		}{
			{"invalid operation type", `{"type":"hold"}`},
			{"malformed JSON", `not json`},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				resp, err := http.Post(
					fmt.Sprintf("%s/wallets/%s/stocks/AAPL", baseURL(), wid),
					"application/json",
					bytes.NewBufferString(tt.body),
				)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			})
		}
	})

	t.Run("returns error status codes", func(t *testing.T) {
		tests := []struct {
			name       string
			setup      func(t *testing.T) string // returns stock name to buy
			wantStatus int
		}{
			{
				name: "stock not found returns 404",
				setup: func(_ *testing.T) string {
					return "DEFINITELY_NOT_A_STOCK_XYZ"
				},
				wantStatus: http.StatusNotFound,
			},
			{
				name: "no bank stock returns 400",
				setup: func(t *testing.T) string {
					setStocks(t, []models.Stock{{Name: "AAPL", Quantity: 0}})
					return "AAPL"
				},
				wantStatus: http.StatusBadRequest,
			},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				stock := tt.setup(t)
				wid := uniqueWalletID(t)
				resp, err := http.Post(
					fmt.Sprintf("%s/wallets/%s/stocks/%s", baseURL(), wid, stock),
					"application/json",
					bytes.NewBufferString(`{"type":"buy"}`),
				)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, tt.wantStatus, resp.StatusCode)
			})
		}
	})
}

// -------------------------------------------------------
// Sell
// -------------------------------------------------------

func TestE2E_Sell(t *testing.T) {
	t.Run("returns 200 and removes stock from wallet", func(t *testing.T) {
		setStocks(t, []models.Stock{{Name: "AAPL", Quantity: 10}})
		wid := uniqueWalletID(t)

		buyStock(t, wid, "AAPL")
		assert.Equal(t, http.StatusOK, sellStock(t, wid, "AAPL"))

		resp, err := http.Get(fmt.Sprintf("%s/wallets/%s", baseURL(), wid))
		require.NoError(t, err)
		defer resp.Body.Close()

		var wallet models.Wallet
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&wallet))
		assert.Empty(t, wallet.Stocks, "expected empty wallet after selling all")
	})

	t.Run("restores bank balance", func(t *testing.T) {
		setStocks(t, []models.Stock{{Name: "AAPL", Quantity: 5}})
		wid := uniqueWalletID(t)

		buyStock(t, wid, "AAPL")
		buyStock(t, wid, "AAPL")
		sellStock(t, wid, "AAPL")

		resp, err := http.Get(baseURL() + "/stocks")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result struct {
			Stocks []models.Stock `json:"stocks"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		stockMap := make(map[string]int64)
		for _, s := range result.Stocks {
			stockMap[s.Name] = s.Quantity
		}
		assert.Equal(t, int64(4), stockMap["AAPL"], "expected bank AAPL=4 (started 5, bought 2, sold 1)")
	})

	t.Run("returns error status codes", func(t *testing.T) {
		tests := []struct {
			name       string
			setup      func(t *testing.T) (walletID, stock string)
			wantStatus int
		}{
			{
				name: "stock not found returns 404",
				setup: func(t *testing.T) (string, string) {
					setStocks(t, []models.Stock{{Name: "AAPL", Quantity: 1}})
					wid := uniqueWalletID(t)
					buyStock(t, wid, "AAPL")
					return wid, "DEFINITELY_NOT_A_STOCK_XYZ"
				},
				wantStatus: http.StatusNotFound,
			},
			{
				name: "no wallet stock returns 400",
				setup: func(t *testing.T) (string, string) {
					setStocks(t, []models.Stock{{Name: "AAPL", Quantity: 5}})
					wid := uniqueWalletID(t)
					buyStock(t, wid, "AAPL")
					sellStock(t, wid, "AAPL")
					return wid, "AAPL"
				},
				wantStatus: http.StatusBadRequest,
			},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				wid, stock := tt.setup(t)
				resp, err := http.Post(
					fmt.Sprintf("%s/wallets/%s/stocks/%s", baseURL(), wid, stock),
					"application/json",
					bytes.NewBufferString(`{"type":"sell"}`),
				)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, tt.wantStatus, resp.StatusCode)
			})
		}
	})
}

// -------------------------------------------------------
// Get wallet stock quantity
// -------------------------------------------------------

func TestE2E_GetWalletStockQuantity(t *testing.T) {
	t.Run("returns quantities", func(t *testing.T) {
		setStocks(t, []models.Stock{
			{Name: "AAPL", Quantity: 10},
			{Name: "GOOG", Quantity: 5},
		})
		wid := uniqueWalletID(t)
		buyStock(t, wid, "AAPL")
		buyStock(t, wid, "AAPL")

		tests := []struct {
			name     string
			stock    string
			wantBody string
		}{
			{"owned stock returns count", "AAPL", "2"},
			{"unowned stock returns zero", "GOOG", "0"},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				resp, err := http.Get(fmt.Sprintf("%s/wallets/%s/stocks/%s", baseURL(), wid, tt.stock))
				require.NoError(t, err)
				defer resp.Body.Close()
				require.Equal(t, http.StatusOK, resp.StatusCode)
				body, _ := io.ReadAll(resp.Body)
				assert.Equal(t, tt.wantBody, strings.TrimSpace(string(body)))
			})
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		setStocks(t, []models.Stock{{Name: "AAPL", Quantity: 5}})
		wid := uniqueWalletID(t)
		buyStock(t, wid, "AAPL")

		tests := []struct {
			name string
			path string
		}{
			{
				name: "wallet not found",
				path: fmt.Sprintf("%s/wallets/definitely-does-not-exist-xyz-123/stocks/AAPL", baseURL()),
			},
			{
				name: "stock not found",
				path: fmt.Sprintf("%s/wallets/%s/stocks/DEFINITELY_NOT_A_STOCK_XYZ", baseURL(), wid),
			},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				resp, err := http.Get(tt.path)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			})
		}
	})
}

// -------------------------------------------------------
// Get wallet (not found)
// -------------------------------------------------------

func TestE2E_GetWallet_NotFound(t *testing.T) {
	resp, err := http.Get(fmt.Sprintf("%s/wallets/definitely-does-not-exist-xyz-123", baseURL()))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// -------------------------------------------------------
// Audit log
// -------------------------------------------------------

func TestE2E_AuditLog(t *testing.T) {
	t.Run("returns 200 with log field", func(t *testing.T) {
		resp, err := http.Get(baseURL() + "/log")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		assert.Contains(t, result, "log")
	})

	t.Run("records buy and sell", func(t *testing.T) {
		setStocks(t, []models.Stock{{Name: "AAPL", Quantity: 5}})
		wid := uniqueWalletID(t)

		buyStock(t, wid, "AAPL")
		sellStock(t, wid, "AAPL")

		resp, err := http.Get(baseURL() + "/log")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		var result struct {
			Log []models.AuditLogEntry `json:"log"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		// Filter to this test's wallet to avoid interference from other tests.
		var walletEntries []models.AuditLogEntry
		for _, e := range result.Log {
			if e.WalletID == wid {
				walletEntries = append(walletEntries, e)
			}
		}
		require.Len(t, walletEntries, 2)
		assert.Equal(t, "buy", walletEntries[0].Type)
		assert.Equal(t, "AAPL", walletEntries[0].StockName)
		assert.Equal(t, "sell", walletEntries[1].Type)
		assert.Equal(t, "AAPL", walletEntries[1].StockName)
	})

	t.Run("excludes failed operations", func(t *testing.T) {
		setStocks(t, []models.Stock{{Name: "AAPL", Quantity: 0}})
		wid := uniqueWalletID(t)

		// Failed buy (bank empty).
		buyStock(t, wid, "AAPL")

		resp, err := http.Get(baseURL() + "/log")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result struct {
			Log []models.AuditLogEntry `json:"log"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		var entries []models.AuditLogEntry
		for _, e := range result.Log {
			if e.WalletID == wid {
				entries = append(entries, e)
			}
		}
		assert.Empty(t, entries, "no log entries expected for failed operation")
	})
}

// -------------------------------------------------------
// Full workflow
// -------------------------------------------------------

func getBank(t *testing.T) map[string]int64 {
	t.Helper()
	resp, err := http.Get(baseURL() + "/stocks")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got struct {
		Stocks []models.Stock `json:"stocks"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	return stocksToMap(got.Stocks)
}

func getWallet(t *testing.T, walletID string) (int, map[string]int64) {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("%s/wallets/%s", baseURL(), walletID))
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}
	var wallet models.Wallet
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&wallet))
	return resp.StatusCode, stocksToMap(wallet.Stocks)
}

func getWalletStock(t *testing.T, walletID, stock string) (int, string) {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("%s/wallets/%s/stocks/%s", baseURL(), walletID, stock))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, strings.TrimSpace(string(body))
}

func TestE2E_FullWorkflow(t *testing.T) {
	// ── 1. Seed the bank with two stock types ──────────────────────────────────
	t.Log("step 1: seeding bank AAPL=10 GOOG=5")
	setStocks(t, []models.Stock{
		{Name: "AAPL", Quantity: 10},
		{Name: "GOOG", Quantity: 5},
	})

	bank := getBank(t)
	t.Logf("bank after seed: %v", bank)
	assert.Equal(t, int64(10), bank["AAPL"])
	assert.Equal(t, int64(5), bank["GOOG"])

	// ── 2. Two wallets perform buy operations ──────────────────────────────────
	walletA := uniqueWalletID(t)
	walletB := uniqueWalletID(t)
	// t.Logf("walletA=%s walletB=%s", walletA, walletB)

	for i := 1; i <= 3; i++ {
		status := buyStock(t, walletA, "AAPL")
		// t.Logf("walletA buy AAPL #%d → %d", i, status)
		require.Equal(t, http.StatusOK, status)
	}
	for i := 1; i <= 2; i++ {
		status := buyStock(t, walletA, "GOOG")
		// t.Logf("walletA buy GOOG #%d → %d", i, status)
		require.Equal(t, http.StatusOK, status)
	}
	for i := 1; i <= 2; i++ {
		status := buyStock(t, walletB, "AAPL")
		// t.Logf("walletB buy AAPL #%d → %d", i, status)
		require.Equal(t, http.StatusOK, status)
	}

	// ── 3. Verify wallet A state ───────────────────────────────────────────────
	statusA, wmA := getWallet(t, walletA)
	// t.Logf("walletA GET → status=%d stocks=%v", statusA, wmA)
	require.Equal(t, http.StatusOK, statusA)
	assert.Equal(t, int64(3), wmA["AAPL"], "walletA should hold 3 AAPL")
	assert.Equal(t, int64(2), wmA["GOOG"], "walletA should hold 2 GOOG")

	statusB, wmB := getWallet(t, walletB)
	// t.Logf("walletB GET → status=%d stocks=%v", statusB, wmB)
	require.Equal(t, http.StatusOK, statusB)
	assert.Equal(t, int64(2), wmB["AAPL"], "walletB should hold 2 AAPL")

	// ── 4. Verify individual stock quantity endpoint ────────────────────────────
	s, body := getWalletStock(t, walletA, "AAPL")
	// t.Logf("walletA /stocks/AAPL → status=%d body=%q", s, body)
	require.Equal(t, http.StatusOK, s)
	assert.Equal(t, "3", body)

	s, body = getWalletStock(t, walletB, "GOOG")
	// t.Logf("walletB /stocks/GOOG (not owned) → status=%d body=%q", s, body)
	require.Equal(t, http.StatusOK, s, "unowned-but-known stock should return 0, not 404")
	assert.Equal(t, "0", body)

	// ── 5. Verify bank after buys ─────────────────────────────────────────────
	// AAPL: 10 − 3(A) − 2(B) = 5; GOOG: 5 − 2(A) = 3
	bank = getBank(t)
	// t.Logf("bank after buys: %v", bank)
	assert.Equal(t, int64(5), bank["AAPL"], "bank AAPL after buys")
	assert.Equal(t, int64(3), bank["GOOG"], "bank GOOG after buys")

	// ── 6. Failed operations ───────────────────────────────────────────────────
	s = buyStock(t, walletA, "UNKNOWN_XYZ")
	// t.Logf("buy unknown stock → %d (want 404)", s)
	assert.Equal(t, http.StatusNotFound, s)

	s = sellStock(t, walletB, "GOOG")
	// t.Logf("walletB sell GOOG (not owned) → %d (want 400)", s)
	assert.Equal(t, http.StatusBadRequest, s)

	// t.Log("resetting bank to add ZERO stock (quantity 0)")
	setStocks(t, []models.Stock{
		{Name: "AAPL", Quantity: 5},
		{Name: "GOOG", Quantity: 3},
		{Name: "ZERO", Quantity: 0},
	})
	bank = getBank(t)
	// t.Logf("bank after second setStocks: %v", bank)

	s = buyStock(t, walletA, "ZERO")
	// t.Logf("buy zero-quantity stock → %d (want 400)", s)
	assert.Equal(t, http.StatusBadRequest, s)

	// ── 7. Sell operations ─────────────────────────────────────────────────────
	// wallet holdings are unchanged by setStocks; only bank was reset.
	s = sellStock(t, walletA, "AAPL")
	// t.Logf("walletA sell AAPL #1 → %d", s)
	require.Equal(t, http.StatusOK, s)

	s = sellStock(t, walletA, "GOOG")
	// t.Logf("walletA sell GOOG #1 → %d", s)
	require.Equal(t, http.StatusOK, s)

	s = sellStock(t, walletA, "GOOG")
	// t.Logf("walletA sell GOOG #2 → %d", s)
	require.Equal(t, http.StatusOK, s)

	// ── 8. Verify wallet A after sells ────────────────────────────────────────
	statusA, wmA = getWallet(t, walletA)
	// t.Logf("walletA after sells → status=%d stocks=%v", statusA, wmA)
	require.Equal(t, http.StatusOK, statusA)
	assert.Equal(t, int64(2), wmA["AAPL"], "walletA: 3 bought − 1 sold = 2 AAPL")
	assert.Equal(t, int64(0), wmA["GOOG"], "walletA: sold all GOOG")

	// ── 9. Verify bank after sells ────────────────────────────────────────────
	// Bank AAPL: reset to 5, +1 sold back = 6. Bank GOOG: reset to 3, +2 sold back = 5.
	bank = getBank(t)
	// t.Logf("bank after sells: %v", bank)
	assert.Equal(t, int64(6), bank["AAPL"], "bank AAPL after sells")
	assert.Equal(t, int64(5), bank["GOOG"], "bank GOOG after sells")

	// ── 10. Non-existent wallet returns 404 ───────────────────────────────────
	statusX, _ := getWallet(t, "no-such-wallet-xyz-123")
	// t.Logf("GET non-existent wallet → %d (want 404)", statusX)
	assert.Equal(t, http.StatusNotFound, statusX)

	// ── 11. Audit log ─────────────────────────────────────────────────────────
	resp, err := http.Get(baseURL() + "/log")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var logResult struct {
		Log []models.AuditLogEntry `json:"log"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&logResult))
	// t.Logf("total audit log entries (all wallets): %d", len(logResult.Log))

	testWallets := map[string]bool{walletA: true, walletB: true}
	var entries []models.AuditLogEntry
	for _, e := range logResult.Log {
		if testWallets[e.WalletID] {
			entries = append(entries, e)
		}
	}
	// t.Logf("audit log entries for this test's wallets (%d):", len(entries))
	// for i, e := range entries {
	// 	t.Logf("  [%d] type=%s wallet=%s stock=%s", i, e.Type, e.WalletID, e.StockName)
	// }

	// Expected: walletA buy AAPL×3 + buy GOOG×2 + sell AAPL×1 + sell GOOG×2 = 8
	//           walletB buy AAPL×2 = 2   → total 10
	require.Len(t, entries, 10, "expected 10 successful operations in audit log")

	var buys, sells int
	for _, e := range entries {
		switch e.Type {
		case "buy":
			buys++
		case "sell":
			sells++
		}
	}
	// t.Logf("audit log: buys=%d sells=%d", buys, sells)
	assert.Equal(t, 7, buys, "7 successful buys total")
	assert.Equal(t, 3, sells, "3 successful sells total")

	for _, e := range entries {
		assert.NotEqual(t, "UNKNOWN_XYZ", e.StockName, "failed buy of unknown stock must not appear")
		assert.NotEqual(t, "ZERO", e.StockName, "failed buy of zero-stock must not appear")
	}
}

// stocksToMap converts a slice of Stock into a name→quantity map for easy assertions.
func stocksToMap(stocks []models.Stock) map[string]int64 {
	m := make(map[string]int64, len(stocks))
	for _, s := range stocks {
		m[s.Name] = s.Quantity
	}
	return m
}

// -------------------------------------------------------
// High availability: service survives chaos
// -------------------------------------------------------

func TestE2E_Chaos_ServiceRemainsAvailableAfterInstanceKill(t *testing.T) {
	// The /chaos endpoint terminates one instance. With 3 instances behind nginx,
	// the remaining two keep serving requests.
	resp, err := http.Post(baseURL()+"/chaos", "application/json", nil)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "chaos should return 200")

	// Allow time for the instance to die and nginx to detect the failure.
	time.Sleep(300 * time.Millisecond)

	// Service must still be accessible — retry up to 5 times.
	client := &http.Client{Timeout: 2 * time.Second}
	var lastErr error
	for i := 0; i < 5; i++ {
		healthResp, err := client.Get(baseURL() + "/health")
		if err == nil && healthResp.StatusCode == http.StatusOK {
			healthResp.Body.Close()
			return
		}
		if healthResp != nil {
			lastErr = fmt.Errorf("status %d", healthResp.StatusCode)
			healthResp.Body.Close()
		} else {
			lastErr = err
		}
		time.Sleep(200 * time.Millisecond)
	}
	require.NoError(t, lastErr, "service should remain available after chaos")
}
