package handlers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// countingDriver is a test-only database/sql driver that counts Exec calls and
// can be told to fail them.
type countingDriver struct {
	execs atomic.Int64
	fail  atomic.Bool
}

func (d *countingDriver) Open(string) (driver.Conn, error) { return &countingConn{driver: d}, nil }

type countingConn struct{ driver *countingDriver }

func (c *countingConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}
func (c *countingConn) Close() error              { return nil }
func (c *countingConn) Begin() (driver.Tx, error) { return nil, errors.New("tx not supported") }
func (c *countingConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	c.driver.execs.Add(1)
	if c.driver.fail.Load() {
		return nil, errors.New("database unavailable")
	}
	return driver.RowsAffected(0), nil
}

var registerCountingDriver sync.Once

func TestEnsureOwnerSchemaRunsOncePerDatabaseHandle(t *testing.T) {
	counter := &countingDriver{}
	name := "koschei-owner-schema-counting"
	registerCountingDriver.Do(func() { sql.Register(name, counter) })
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); ownerSchemaEnsured.Delete(db) })
	ctx := context.Background()

	counter.fail.Store(true)
	if err := ensureOwnerSchema(ctx, db); err == nil {
		t.Fatal("a failing schema apply must be reported")
	}
	if _, done := ownerSchemaEnsured.Load(db); done {
		t.Fatal("a failed schema apply must not be remembered")
	}

	counter.fail.Store(false)
	counter.execs.Store(0)
	if err := ensureOwnerSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	applied := counter.execs.Load()
	if applied == 0 {
		t.Fatal("the first successful call must apply the schema")
	}
	for i := 0; i < 3; i++ {
		if err := ensureOwnerSchema(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	if got := counter.execs.Load(); got != applied {
		t.Fatalf("schema re-applied on later sign-ins: %d statements after the first %d", got-applied, applied)
	}
}
