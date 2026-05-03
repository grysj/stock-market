package e2e_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"remitly_task/internal/models"
)

func TestE2E_Concurrent(t *testing.T) {
	t.Run("never oversells", func(t *testing.T) {
		const initial = 5
		const goroutines = 20
		setStocks(t, []models.Stock{{Name: "AAPL", Quantity: initial}})

		wids := make([]string, goroutines)
		for i := range wids {
			wids[i] = fmt.Sprintf("%s-%d", uniqueWalletID(t), i)
		}

		var wg sync.WaitGroup
		var bought int64
		for _, wid := range wids {
			wg.Add(1)
			go func(wid string) {
				defer wg.Done()
				if buyStock(t, wid, "AAPL") == http.StatusOK {
					atomic.AddInt64(&bought, 1)
				}
			}(wid)
		}
		wg.Wait()

		assert.Equal(t, int64(initial), bought, "successful buys must equal initial bank quantity")

		resp, err := http.Get(baseURL() + "/stocks")
		require.NoError(t, err)
		defer resp.Body.Close()
		var result struct {
			Stocks []models.Stock `json:"stocks"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		for _, s := range result.Stocks {
			if s.Name == "AAPL" {
				assert.Equal(t, int64(0), s.Quantity, "bank must be empty after all successful buys")
			}
		}
	})

	t.Run("quantity is conserved", func(t *testing.T) {
		const initial int64 = 20
		const goroutines = 10
		setStocks(t, []models.Stock{{Name: "AAPL", Quantity: initial}})

		wids := make([]string, goroutines)
		for i := range wids {
			wids[i] = fmt.Sprintf("%s-%d", uniqueWalletID(t), i)
		}

		var wg sync.WaitGroup
		for _, wid := range wids {
			wg.Add(1)
			go func(wid string) {
				defer wg.Done()
				buyStock(t, wid, "AAPL")
				buyStock(t, wid, "AAPL")
				sellStock(t, wid, "AAPL")
			}(wid)
		}
		wg.Wait()

		resp, err := http.Get(baseURL() + "/stocks")
		require.NoError(t, err)
		defer resp.Body.Close()
		var stocksResult struct {
			Stocks []models.Stock `json:"stocks"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&stocksResult))

		var total int64
		for _, s := range stocksResult.Stocks {
			if s.Name == "AAPL" {
				total += s.Quantity
			}
		}
		for _, wid := range wids {
			r, err := http.Get(fmt.Sprintf("%s/wallets/%s/stocks/AAPL", baseURL(), wid))
			require.NoError(t, err)
			body, _ := io.ReadAll(r.Body)
			r.Body.Close()
			if r.StatusCode == http.StatusOK {
				var qty int64
				fmt.Sscan(strings.TrimSpace(string(body)), &qty)
				total += qty
			}
			// 404 means wallet holds 0 of this stock — contributes nothing to total.
		}

		assert.Equal(t, initial, total, "total quantity across bank and all wallets must equal initial")
	})

	t.Run("sell never goes negative", func(t *testing.T) {
		const bankQty = 5
		setStocks(t, []models.Stock{{Name: "AAPL", Quantity: bankQty}})
		wid := uniqueWalletID(t)

		for i := 0; i < bankQty; i++ {
			require.Equal(t, http.StatusOK, buyStock(t, wid, "AAPL"), "pre-buy %d", i)
		}

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sellStock(t, wid, "AAPL")
			}()
		}
		wg.Wait()

		resp, err := http.Get(fmt.Sprintf("%s/wallets/%s/stocks/AAPL", baseURL(), wid))
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			var qty int64
			fmt.Sscan(strings.TrimSpace(string(body)), &qty)
			assert.GreaterOrEqual(t, qty, int64(0), "wallet quantity must not go below zero")
		} else {
			assert.Equal(t, http.StatusNotFound, resp.StatusCode, "404 is valid when all stock has been sold")
		}
	})
}
