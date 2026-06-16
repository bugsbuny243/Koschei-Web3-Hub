package http

import (
	"database/sql"
	"net"
	"net/http"
	"strings"
	"sync"
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
	if r == nil {
		return "unknown"
	}
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" && len(xff) < 256 {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if strings.TrimSpace(r.RemoteAddr) != "" && len(r.RemoteAddr) < 128 {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return "unknown"
}
