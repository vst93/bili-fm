package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func testDB(t *testing.T, path string) *DB {
	t.Helper()
	if testing.Short() {
		t.Skip("requires the plugin's native SQLite library; run go generate ./plugins/sqlite")
	}
	db, err := Open(context.Background(), path, OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func execute(t *testing.T, db *DB, sql string, args ...any) Result {
	t.Helper()
	r, err := db.Execute(context.Background(), sql, args...)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func query(t *testing.T, db *DB, sql string, args ...any) Rows {
	t.Helper()
	r, err := db.Query(context.Background(), sql, args...)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestValues(t *testing.T) {
	db := testDB(t, ":memory:")
	values := []any{nil, int64(math.MinInt64), int64(math.MaxInt64), -3.125, "中文🙂\x00text", []byte{0, 1, 255}, []byte{}, "", true}
	rows := query(t, db, "SELECT ? AS same, ? AS same, ?, ?, ?, ?, ?, ?, ?", values...)
	want := append([]any{}, values...)
	want[8] = int64(1)
	if !reflect.DeepEqual(rows.Values, [][]any{want}) {
		t.Fatalf("values = %#v, want %#v", rows.Values, want)
	}
	if rows.Columns[0] != "same" || rows.Columns[1] != "same" {
		t.Fatal(rows.Columns)
	}
	execute(t, db, "CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT UNIQUE, bytes BLOB)")
	r := execute(t, db, "INSERT INTO items(name, bytes) VALUES (?, ?)", "it's a ?; DROP TABLE items; --", []byte{1, 2})
	if r.Changes != 1 || r.LastInsertID != 1 {
		t.Fatal(r)
	}
	if n := execute(t, db, "CREATE TABLE other (n)").Changes; n != 0 {
		t.Fatalf("DDL changes = %d", n)
	}
	got := query(t, db, "SELECT name, bytes FROM items").Values[0]
	if got[0] != "it's a ?; DROP TABLE items; --" || !reflect.DeepEqual(got[1], []byte{1, 2}) {
		t.Fatal(got)
	}
	// BLOB storage belongs to Go after SQLite finalizes the statement.
	_ = query(t, db, "SELECT randomblob(65536)")
	if !reflect.DeepEqual(got[1], []byte{1, 2}) {
		t.Fatal("BLOB changed after a later step")
	}
	if rows := query(t, db, "SELECT * FROM items WHERE 0"); len(rows.Values) != 0 || len(rows.Columns) != 3 {
		t.Fatal(rows)
	}
	if r := query(t, db, "INSERT INTO items(name) VALUES ('returning') RETURNING id"); r.Values[0][0] != int64(2) {
		t.Fatal(r)
	}
	if r := query(t, db, "SELECT ?, ?, ?", json.Number("42"), json.Number("0.25"), uint32(7)); !reflect.DeepEqual(r.Values[0], []any{int64(42), 0.25, int64(7)}) {
		t.Fatal(r)
	}
}

func TestSQLiteFeatures(t *testing.T) {
	db := testDB(t, ":memory:")
	if v := query(t, db, "SELECT sqlite_version()").Values[0][0]; v != "3.53.4" {
		t.Fatalf("SQLite = %v", v)
	}
	execute(t, db, "CREATE TABLE parent (id PRIMARY KEY)")
	execute(t, db, "CREATE TABLE child (parent REFERENCES parent(id))")
	_, err := db.Execute(context.Background(), "INSERT INTO child VALUES (1)")
	var sqliteErr *Error
	if !errors.As(err, &sqliteErr) || sqliteErr.Code != 19 || sqliteErr.ExtendedCode != 787 {
		t.Fatalf("foreign key: %v", err)
	}
	execute(t, db, "CREATE VIRTUAL TABLE search USING fts5(body)")
	execute(t, db, "INSERT INTO search VALUES ('hello sqlite')")
	if n := query(t, db, "SELECT count(*) FROM search WHERE search MATCH 'sqlite'").Values[0][0]; n != int64(1) {
		t.Fatal(n)
	}
	execute(t, db, "CREATE VIRTUAL TABLE spatial USING rtree(id, x0, x1, y0, y1)")
	if v := query(t, db, `SELECT json_extract('{"a":42}', '$.a')`).Values[0][0]; v != int64(42) {
		t.Fatal(v)
	}
	execute(t, db, "VACUUM")
}

func TestValidationBeforeExecution(t *testing.T) {
	db := testDB(t, ":memory:")
	execute(t, db, "CREATE TABLE items (n UNIQUE)")
	for _, sql := range []string{"INSERT INTO items VALUES (1); INSERT INTO items VALUES (2)", "INSERT INTO items VALUES (3); invalid sql", "INSERT INTO items VALUES (4)\x00;", "", "-- only a comment", "BEGIN", "SAVEPOINT foo", "ATTACH ':memory:' AS other", "VACUUM INTO 'outside.db'"} {
		if _, err := db.Execute(context.Background(), sql); err == nil {
			t.Errorf("accepted %q", sql)
		}
	}
	if n := query(t, db, "SELECT count(*) FROM items").Values[0][0]; n != int64(0) {
		t.Fatalf("partial SQL ran: %v", n)
	}
	execute(t, db, "INSERT INTO items VALUES (?) ; -- trailing comment\n /* another */", 1)
	for _, args := range [][]any{nil, {1, 2}, {math.NaN()}, {math.Inf(1)}, {uint64(math.MaxUint64)}, {json.Number("9223372036854775808")}, {struct{}{}}, {"\xff"}} {
		if _, err := db.Execute(context.Background(), "INSERT INTO items VALUES (?)", args...); err == nil {
			t.Errorf("accepted args %#v", args)
		}
	}
}

func TestTransactions(t *testing.T) {
	db := testDB(t, ":memory:")
	execute(t, db, "CREATE TABLE items (n UNIQUE)")
	results, err := db.Transaction(context.Background(), []Statement{{"INSERT INTO items VALUES (?)", []any{1}}, {"INSERT INTO items VALUES (?)", []any{2}}})
	if err != nil || len(results) != 2 || results[0].Changes != 1 || results[1].Changes != 1 {
		t.Fatalf("%v, %v", results, err)
	}
	for _, second := range []string{"INSERT INTO items VALUES (2)", "COMMIT", "SAVEPOINT escape", "not sql"} {
		results, err := db.Transaction(context.Background(), []Statement{{SQL: "INSERT INTO items VALUES (3)"}, {SQL: second}})
		if err == nil || results != nil {
			t.Fatalf("batch succeeded: %q: %v %v", second, results, err)
		}
		if n := query(t, db, "SELECT count(*) FROM items").Values[0][0]; n != int64(2) {
			t.Fatalf("batch did not roll back: %q: %v", second, n)
		}
	}
	if r, err := db.Transaction(context.Background(), nil); err != nil || r == nil || len(r) != 0 {
		t.Fatalf("empty batch: %v %v", r, err)
	}
}

const longQuery = "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<1000000000) SELECT sum(x) FROM n"

func TestCancellationAndRollback(t *testing.T) {
	db := testDB(t, ":memory:")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := db.Query(ctx, longQuery); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
	if n := query(t, db, "SELECT 42").Values[0][0]; n != int64(42) {
		t.Fatal(n)
	}
	execute(t, db, "CREATE TABLE items (n)")
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := db.Transaction(ctx, []Statement{{SQL: "INSERT INTO items VALUES (1)"}, {SQL: longQuery}}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("batch cancellation: %v", err)
	}
	if n := query(t, db, "SELECT count(*) FROM items").Values[0][0]; n != int64(0) {
		t.Fatal("canceled batch did not roll back", n)
	}
}

func TestConcurrentOperations(t *testing.T) {
	db := testDB(t, ":memory:")
	execute(t, db, "CREATE TABLE counter(n)")
	execute(t, db, "INSERT INTO counter VALUES (0)")
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 8 {
				if _, err := db.Execute(context.Background(), "UPDATE counter SET n=n+1"); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
	if n := query(t, db, "SELECT n FROM counter").Values[0][0]; n != int64(128) {
		t.Fatal(n)
	}
}

func TestCloseAndQueuedCancellation(t *testing.T) {
	db := testDB(t, ":memory:")
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- db.operation(context.Background(), func() error { close(started); _, _, err := db.run(longQuery, nil, true); return err })
	}()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := db.Query(ctx, "SELECT 1"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued cancellation: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrClosed) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not interrupt query")
	}
	if _, err := db.Query(context.Background(), "SELECT 1"); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDiskReadOnlyAndBusy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk.db")
	w := testDB(t, path)
	execute(t, w, "CREATE TABLE items(n)")
	execute(t, w, "INSERT INTO items VALUES (42)")
	r, err := Open(context.Background(), path, OpenOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if n := query(t, r, "SELECT n FROM items").Values[0][0]; n != int64(42) {
		t.Fatal(n)
	}
	if _, err := r.Execute(context.Background(), "DELETE FROM items"); err == nil {
		t.Fatal("read-only write succeeded")
	}
	b, err := Open(context.Background(), path, OpenOptions{BusyTimeout: 30 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	locked, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- w.operation(context.Background(), func() error {
			if err := w.control("BEGIN IMMEDIATE"); err != nil {
				close(locked)
				return err
			}
			close(locked)
			<-release
			return w.control("COMMIT")
		})
	}()
	<-locked
	defer func() {
		close(release)
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	_, err = b.Execute(context.Background(), "INSERT INTO items VALUES (1)")
	var sqliteErr *Error
	if !errors.As(err, &sqliteErr) || sqliteErr.Code != 5 {
		t.Fatalf("busy: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := b.Execute(ctx, "INSERT INTO items VALUES (1)"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("busy cancellation: %v", err)
	}
}
