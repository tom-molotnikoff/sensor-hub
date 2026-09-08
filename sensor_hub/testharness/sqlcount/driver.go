package sqlcount

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync"
	"sync/atomic"

	_ "modernc.org/sqlite"
)

var registrations atomic.Int64

type Transaction struct {
	Statements []string
	Committed  bool
	RolledBack bool
}

type Recorder struct {
	mu           sync.Mutex
	transactions []*Transaction
	loose        []string
}

func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transactions = nil
	r.loose = nil
}

func (r *Recorder) Transactions() []Transaction {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Transaction, 0, len(r.transactions))
	for _, tx := range r.transactions {
		out = append(out, Transaction{
			Statements: append([]string(nil), tx.Statements...),
			Committed:  tx.Committed,
			RolledBack: tx.RolledBack,
		})
	}
	return out
}

func (r *Recorder) Loose() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.loose...)
}

func (r *Recorder) All() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	all := append([]string(nil), r.loose...)
	for _, tx := range r.transactions {
		all = append(all, tx.Statements...)
	}
	return all
}

func (r *Recorder) begin() *Transaction {
	tx := &Transaction{}
	r.mu.Lock()
	r.transactions = append(r.transactions, tx)
	r.mu.Unlock()
	return tx
}

func (r *Recorder) statement(tx *Transaction, query string) {
	r.mu.Lock()
	if tx == nil {
		r.loose = append(r.loose, query)
	} else {
		tx.Statements = append(tx.Statements, query)
	}
	r.mu.Unlock()
}

func (r *Recorder) settle(tx *Transaction, committed bool) {
	r.mu.Lock()
	tx.Committed = committed
	tx.RolledBack = !committed
	r.mu.Unlock()
}

func Register() (string, *Recorder, error) {
	probe, err := sql.Open("sqlite", "")
	if err != nil {
		return "", nil, fmt.Errorf("could not open the base sqlite driver: %w", err)
	}
	defer probe.Close()

	recorder := &Recorder{}
	name := fmt.Sprintf("sqlite-counting-%d", registrations.Add(1))
	sql.Register(name, recordingDriver{base: probe.Driver(), recorder: recorder})
	return name, recorder, nil
}

type recordingDriver struct {
	base     driver.Driver
	recorder *Recorder
}

func (d recordingDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &recordingConn{inner: conn, recorder: d.recorder}, nil
}

type recordingConn struct {
	inner    driver.Conn
	recorder *Recorder
	mu       sync.Mutex
	current  *Transaction
}

func (c *recordingConn) openTransaction() *Transaction {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

func (c *recordingConn) setTransaction(tx *Transaction) {
	c.mu.Lock()
	c.current = tx
	c.mu.Unlock()
}

func (c *recordingConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.inner.Prepare(query)
	if err != nil {
		return nil, err
	}
	return recordingStmt{inner: stmt, query: query, conn: c}, nil
}

func (c *recordingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	preparer, ok := c.inner.(driver.ConnPrepareContext)
	if !ok {
		return c.Prepare(query)
	}
	stmt, err := preparer.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return recordingStmt{inner: stmt, query: query, conn: c}, nil
}

func (c *recordingConn) Close() error { return c.inner.Close() }

func (c *recordingConn) Begin() (driver.Tx, error) {
	tx, err := c.inner.Begin()
	if err != nil {
		return nil, err
	}
	return c.opened(tx), nil
}

func (c *recordingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	beginner, ok := c.inner.(driver.ConnBeginTx)
	if !ok {
		return c.Begin()
	}
	tx, err := beginner.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return c.opened(tx), nil
}

func (c *recordingConn) opened(inner driver.Tx) driver.Tx {
	record := c.recorder.begin()
	c.setTransaction(record)
	return recordingTx{inner: inner, conn: c, record: record}
}

type recordingTx struct {
	inner  driver.Tx
	conn   *recordingConn
	record *Transaction
}

func (t recordingTx) Commit() error {
	err := t.inner.Commit()
	t.conn.setTransaction(nil)
	t.conn.recorder.settle(t.record, err == nil)
	return err
}

func (t recordingTx) Rollback() error {
	err := t.inner.Rollback()
	t.conn.setTransaction(nil)
	t.conn.recorder.settle(t.record, false)
	return err
}

type recordingStmt struct {
	inner driver.Stmt
	query string
	conn  *recordingConn
}

func (s recordingStmt) Close() error  { return s.inner.Close() }
func (s recordingStmt) NumInput() int { return s.inner.NumInput() }

func (s recordingStmt) record() {
	s.conn.recorder.statement(s.conn.openTransaction(), s.query)
}

func (s recordingStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.record()
	return s.inner.Exec(args)
}

func (s recordingStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.record()
	return s.inner.Query(args)
}

func (s recordingStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := s.inner.(driver.StmtExecContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	s.record()
	return execer.ExecContext(ctx, args)
}

func (s recordingStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	querier, ok := s.inner.(driver.StmtQueryContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	s.record()
	return querier.QueryContext(ctx, args)
}
