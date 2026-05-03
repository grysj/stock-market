package models

type Stock struct {
	Name     string `json:"name"`
	Quantity int64  `json:"quantity"`
}

type Wallet struct {
	ID     string  `json:"id"`
	Stocks []Stock `json:"stocks"`
}

type AuditLogEntry struct {
	Type      string `json:"type"`
	WalletID  string `json:"wallet_id"`
	StockName string `json:"stock_name"`
}

type WalletStockOperation struct {
	WalletID  string
	StockName string
	Type      string
}

type SetStocks struct {
	Stocks []Stock
}

type StockBalance struct {
	Name     string
	Quantity int64
}
