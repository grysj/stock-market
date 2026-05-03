package response

import "remitly_task/internal/models"

type Stocks struct {
	Stocks []models.Stock `json:"stocks"`
}

type Log struct {
	Log []models.AuditLogEntry `json:"log"`
}

type Error struct {
	Error string `json:"error"`
}

type Health struct {
	Status string `json:"status"`
}

type Chaos struct {
	Message string `json:"message"`
}
