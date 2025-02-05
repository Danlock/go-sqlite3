package tests

import (
	"errors"
	"testing"

	"github.com/ncruces/go-sqlite3"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func TestCreateFunction(t *testing.T) {
	t.Parallel()

	db, err := sqlite3.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.CreateFunction("test", 1, sqlite3.INNOCUOUS, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		switch arg := arg[0]; arg.Int() {
		case 0:
			ctx.ResultInt(arg.Int())
		case 1:
			ctx.ResultInt64(arg.Int64())
		case 2:
			ctx.ResultBool(arg.Bool())
		case 3:
			ctx.ResultFloat(arg.Float())
		case 4:
			ctx.ResultText(arg.Text())
		case 5:
			ctx.ResultBlob(arg.Blob(nil))
		case 6:
			ctx.ResultZeroBlob(arg.Int64())
		case 7:
			ctx.ResultTime(arg.Time(sqlite3.TimeFormatUnix), sqlite3.TimeFormatDefault)
		case 8:
			var v any
			if err := arg.JSON(&v); err != nil {
				ctx.ResultError(err)
			} else {
				ctx.ResultJSON(v)
			}
		case 9:
			ctx.ResultValue(arg)
		case 10:
			ctx.ResultNull()
		case 11:
			ctx.ResultError(sqlite3.FULL)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	stmt, _, err := db.Prepare(`SELECT test(value) FROM generate_series(0)`)
	if err != nil {
		t.Error(err)
	}
	defer stmt.Close()

	if stmt.Step() {
		if got := stmt.ColumnType(0); got != sqlite3.INTEGER {
			t.Errorf("got %v, want INTEGER", got)
		}
		if got := stmt.ColumnInt(0); got != 0 {
			t.Errorf("got %v, want 1", got)
		}
	}

	if stmt.Step() {
		if got := stmt.ColumnType(0); got != sqlite3.INTEGER {
			t.Errorf("got %v, want INTEGER", got)
		}
		if got := stmt.ColumnInt64(0); got != 1 {
			t.Errorf("got %v, want 2", got)
		}
	}

	if stmt.Step() {
		if got := stmt.ColumnType(0); got != sqlite3.INTEGER {
			t.Errorf("got %v, want INTEGER", got)
		}
		if got := stmt.ColumnBool(0); got != true {
			t.Errorf("got %v, want true", got)
		}
	}

	if stmt.Step() {
		if got := stmt.ColumnType(0); got != sqlite3.FLOAT {
			t.Errorf("got %v, want FLOAT", got)
		}
		if got := stmt.ColumnInt64(0); got != 3 {
			t.Errorf("got %v, want 3", got)
		}
	}

	if stmt.Step() {
		if got := stmt.ColumnType(0); got != sqlite3.TEXT {
			t.Errorf("got %v, want TEXT", got)
		}
		if got := stmt.ColumnText(0); got != "4" {
			t.Errorf("got %s, want 4", got)
		}
	}

	if stmt.Step() {
		if got := stmt.ColumnType(0); got != sqlite3.BLOB {
			t.Errorf("got %v, want BLOB", got)
		}
		if got := stmt.ColumnRawBlob(0); string(got) != "5" {
			t.Errorf("got %s, want 5", got)
		}
	}

	if stmt.Step() {
		if got := stmt.ColumnType(0); got != sqlite3.BLOB {
			t.Errorf("got %v, want BLOB", got)
		}
		if got := stmt.ColumnRawBlob(0); len(got) != 6 {
			t.Errorf("got %v, want 6", got)
		}
	}

	if stmt.Step() {
		if got := stmt.ColumnType(0); got != sqlite3.TEXT {
			t.Errorf("got %v, want TEXT", got)
		}
		if got := stmt.ColumnTime(0, sqlite3.TimeFormatAuto); got.Unix() != 7 {
			t.Errorf("got %v, want 7", got)
		}
	}

	if stmt.Step() {
		if got := stmt.ColumnType(0); got != sqlite3.TEXT {
			t.Errorf("got %v, want TEXT", got)
		}
		var got int
		if err := stmt.ColumnJSON(0, &got); err != nil {
			t.Error(err)
		} else if got != 8 {
			t.Errorf("got %v, want 8", got)
		}
	}

	if stmt.Step() {
		if got := stmt.ColumnType(0); got != sqlite3.INTEGER {
			t.Errorf("got %v, want INTEGER", got)
		}
		if got := stmt.ColumnInt64(0); got != 9 {
			t.Errorf("got %v, want 9", got)
		}
	}

	if stmt.Step() {
		if got := stmt.ColumnType(0); got != sqlite3.NULL {
			t.Errorf("got %v, want NULL", got)
		}
	}

	if stmt.Step() {
		t.Error("want error")
	}
	if err := stmt.Err(); !errors.Is(err, sqlite3.FULL) {
		t.Errorf("got %v, want sqlite3.FULL", err)
	}
}

func TestAnyCollationNeeded(t *testing.T) {
	t.Parallel()

	db, err := sqlite3.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.Exec(`CREATE TABLE IF NOT EXISTS users (id INT, name VARCHAR(10))`)
	if err != nil {
		t.Fatal(err)
	}

	err = db.Exec(`INSERT INTO users (id, name) VALUES (0, 'go'), (1, 'zig'), (2, 'whatever')`)
	if err != nil {
		t.Fatal(err)
	}

	db.AnyCollationNeeded()

	stmt, _, err := db.Prepare(`SELECT id, name FROM users ORDER BY name COLLATE silly`)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	row := 0
	ids := []int{0, 2, 1}
	names := []string{"go", "whatever", "zig"}
	for ; stmt.Step(); row++ {
		id := stmt.ColumnInt(0)
		name := stmt.ColumnText(1)

		if id != ids[row] {
			t.Errorf("got %d, want %d", id, ids[row])
		}
		if name != names[row] {
			t.Errorf("got %q, want %q", name, names[row])
		}
	}
	if row != 3 {
		t.Errorf("got %d, want %d", row, len(ids))
	}

	if err := stmt.Err(); err != nil {
		t.Fatal(err)
	}
}
