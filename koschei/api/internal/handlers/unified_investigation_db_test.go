package handlers

import (
	"database/sql"
	"testing"
)

func TestUnifiedInvestigationDBPrefersPrimaryForLiveScan(t *testing.T) {
	primary := &sql.DB{}
	readReplica := &sql.DB{}
	h := &Handler{DB: primary, DBRead: readReplica}

	if got := h.unifiedInvestigationDB(true); got != primary {
		t.Fatalf("live investigation selected %p; want primary %p", got, primary)
	}
}

func TestUnifiedInvestigationDBPrefersReadReplicaForStoredProjection(t *testing.T) {
	primary := &sql.DB{}
	readReplica := &sql.DB{}
	h := &Handler{DB: primary, DBRead: readReplica}

	if got := h.unifiedInvestigationDB(false); got != readReplica {
		t.Fatalf("stored projection selected %p; want read replica %p", got, readReplica)
	}
}

func TestUnifiedInvestigationDBFallsBackWithoutDedicatedReplica(t *testing.T) {
	primary := &sql.DB{}
	h := &Handler{DB: primary}

	if got := h.unifiedInvestigationDB(true); got != primary {
		t.Fatalf("live primary fallback selected %p; want %p", got, primary)
	}
	if got := h.unifiedInvestigationDB(false); got != primary {
		t.Fatalf("stored primary fallback selected %p; want %p", got, primary)
	}
}

func TestUnifiedInvestigationDBAllowsReadOnlyProjectionWithoutPrimary(t *testing.T) {
	readReplica := &sql.DB{}
	h := &Handler{DBRead: readReplica}

	if got := h.unifiedInvestigationDB(false); got != readReplica {
		t.Fatalf("stored read-only projection selected %p; want %p", got, readReplica)
	}
}
