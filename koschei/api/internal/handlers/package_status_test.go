package handlers

import (
	"context"
	"testing"
)

func TestCustomerPackageStatusFailsClosedWithoutDatabase(t *testing.T) {
	h := &Handler{}
	status, err := h.customerPackageStatus(context.Background(), "subject", "member@example.com")
	if err == nil {
		t.Fatal("expected database verification error")
	}
	if status.HasActivePackage {
		t.Fatal("package access must not be granted when the database is unavailable")
	}
}
