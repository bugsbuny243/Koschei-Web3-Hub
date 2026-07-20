package http

import (
	"database/sql"
	"net/http"
	"sync"

	"koschei/api/internal/requestmeta"
)

var securityAuditState struct {
	mu sync.RWMutex
	db *sql.DB
}

func setSecurityAuditDB(db *sql.DB) {
	securityAuditState.mu.Lock()
	securityAuditState.db = db
	securityAuditState.mu.Unlock()
}

func getSecurityAuditDB() *sql.DB {
	securityAuditState.mu.RLock()
	defer securityAuditState.mu.RUnlock()
	return securityAuditState.db
}

func securityClientIP(r *http.Request) string {
	return requestmeta.ClientIP(r)
}
