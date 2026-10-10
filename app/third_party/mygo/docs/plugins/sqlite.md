# SQLite

The SQLite plugin provides local databases to web pages and Go, including
native UI apps. It ships prebuilt SQLite libraries and loads them through
purego, so apps build without cgo or a C compiler. It supports macOS,
Linux and Windows on amd64 and arm64.

## Using the page plugin

Call `mygo.Use(sqlite.Plugin)` only when the web frontend uses
`@mygo-plugins/sqlite`. It registers the Go service called by that
TypeScript client. Go and native UI apps use `sqlite.Open` directly, as
shown [below](#from-go-or-native-ui).

```go
import (
    "github.com/egoist/mygo"
    "github.com/egoist/mygo/plugins/sqlite"
)

func main() {
    mygo.Use(sqlite.Plugin)
    // Create windows and call App.Run as usual.
}
```

Install the frontend package with `bun add @mygo-plugins/sqlite`, then:

```ts
import { open } from "@mygo-plugins/sqlite";

const db = await open("app.db");
await db.execute("CREATE TABLE IF NOT EXISTS notes (id INTEGER PRIMARY KEY, title TEXT)");
const result = await db.execute("INSERT INTO notes(title) VALUES (?)", ["Hello SQLite"]);
const { columns, rows } = await db.query("SELECT id, title FROM notes WHERE id = ?", [result.lastInsertId]);
// columns: ["id", "title"], rows: [[1, "Hello SQLite"]]
await db.close();
```

`open` takes a database file name, such as `app.db`, or `:memory:` for a
private in-memory database. Files live in `mygo.PathUserData`. Absolute
paths, directory traversal, symbolic-link database files and Windows
device names are rejected. To choose the directory and lock wait:

```go
mygo.Use(sqlite.New(sqlite.Options{
    Directory: "/path/to/databases",
    BusyTimeout: 2 * time.Second,
}))
```

`Directory` defaults to `mygo.PathUserData`; a missing directory is created
for writable databases. `BusyTimeout` defaults to five seconds; a negative
duration disables waiting. `open("app.db", { readOnly: true })` opens an
existing file without allowing writes.

## Queries and values

`execute(sql, params?)` accepts exactly one statement, consumes any
`RETURNING` rows, and returns `{ changes, lastInsertId }`. `changes` counts
directly affected rows; `lastInsertId` is SQLite's last inserted rowid on
that connection, always a `bigint` in TypeScript.

`query(sql, params?)` returns `{ columns, rows }`. Each row is an array of
cells in column order, preserving duplicate column names. It supports
`SELECT`, pragmas and statements with `RETURNING`. Results are buffered;
use `LIMIT` and pagination for large tables.

Parameters bind by their SQLite index: `?`, `?NNN`, `:name`, `$name` and
`@name` are accepted, with values supplied in an array. All parameter
indexes must have values; missing or excess parameters fail. Never
interpolate application values into SQL strings. Multiple statements are
rejected before the first executes; a trailing semicolon and comments are
allowed.

| SQLite storage class | TypeScript | Go |
| --- | --- | --- |
| NULL | `null` | `nil` |
| INTEGER | `number` when safe, otherwise `bigint` | `int64` |
| REAL | `number` | `float64` |
| TEXT | `string` | `string` |
| BLOB | `Uint8Array` | `[]byte` |

Boolean parameters become integers 0 or 1. Bind a `bigint` when an integer
exceeds JavaScript's safe range; values outside SQLite's signed 64-bit
range fail. Nonfinite numeric parameters fail. Empty BLOBs and empty text
remain distinct from NULL. Text and BLOB parameters are copied by SQLite,
and result BLOBs are copied before a statement closes.

## Transactions and lifetime

```ts
await db.transaction([
  { sql: "INSERT INTO notes(title) VALUES (?)", params: ["first"] },
  { sql: "INSERT INTO notes(title) VALUES (?)", params: ["second"] },
]);
```

`transaction` returns one execution result per statement after COMMIT. It
uses `BEGIN IMMEDIATE`, holds the connection for the entire batch, and
rolls back on any error or cancellation. An empty batch returns `[]`.
Transaction-control statements (`BEGIN`, `COMMIT`, `ROLLBACK`, `SAVEPOINT`,
`RELEASE`) are rejected in user SQL; use the batch API. `ATTACH`, `DETACH`
and `VACUUM INTO` are rejected so SQL cannot open files outside the
configured directory. Ordinary `VACUUM` is supported. SQLite extensions
cannot be loaded.

Each connection belongs to its calling page. Other windows cannot use its
handle. Page navigation and window close cancel calls and close the page's
connections; app quit closes every remaining connection. `close()` is
idempotent and cancels active operations. Concurrent operations on one
connection serialize; await dependent calls to give them an order.

Cancellation is handled by a progress handler and busy handler in C, with
an atomic cancellation token. SQLite retains no Go pointers or purego
callbacks. Go callers can cancel each operation with its context.

## From Go or native UI

No `mygo.Use(sqlite.Plugin)` registration is needed. Open a connection
directly and close it when finished:

```go
ctx := context.Background()
db, err := sqlite.Open(ctx, "app.db", sqlite.OpenOptions{})
if err != nil { return err }
defer db.Close()

_, err = db.Execute(ctx, "CREATE TABLE IF NOT EXISTS notes (title TEXT)")
if err != nil { return err }
_, err = db.Transaction(ctx, []sqlite.Statement{
    {SQL: "INSERT INTO notes VALUES (?)", Args: []any{"first"}},
    {SQL: "INSERT INTO notes VALUES (?)", Args: []any{"second"}},
})
if err != nil { return err }
rows, err := db.Query(ctx, "SELECT title FROM notes")
// rows.Columns and rows.Values contain copied results.
```

Go's `Open` accepts a path directly; its parent directory must exist.
Connections are safe from any goroutine. Their lifetime belongs to the Go
caller, who must call `Close`. `sqlite.Error` exposes the primary and
extended SQLite error codes; closed connections return `sqlite.ErrClosed`,
and canceled operations return their context error.

<!-- repository-only:start -->

## Building and shipping

Prebuilt libraries are published in
[sqlite-3.53.4-1](https://github.com/egoist/mygo/releases/tag/sqlite-3.53.4-1).
CI, `go test` and unpackaged programs download the matching library and
verify its SHA-256. Testing and building apps require no Zig installation.

When updating SQLite or its C shim, run `go generate ./plugins/sqlite` with
Zig 0.16.0 to build all six native libraries from the pinned,
checksum-verified SQLite 3.53.4 source. It writes the manifest, ignored
release assets and local CLI cache. From `plugins/sqlite`,
`go run ./internal/libbuild -target host` builds just the host for local
development before new release assets are published.

`mygo build` and `mygo dev` discover `mygo-plugin.json` and put the correct
library among the app's resources, signed with the app on macOS. macOS
universal builds merge both architectures. Packaged apps need no runtime
downloads. `MYGO_SQLITE_LIBRARY` names a custom build for development.

See [the plugin README](https://github.com/egoist/mygo/tree/main/plugins/sqlite)
for release-asset publication and compile options. SQLite's upstream
[C API](https://www.sqlite.org/c3ref/intro.html) describes its SQL behavior.

<!-- repository-only:end -->
