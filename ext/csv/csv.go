// Package csv provides a CSV virtual table.
//
// The CSV virtual table reads RFC 4180 formatted comma-separated values,
// and returns that content as if it were rows and columns of an SQL table.
//
// https://sqlite.org/csv.html
package csv

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/internal/util"
)

// Register registers the CSV virtual table.
// If a filename is specified, [os.Open] is used to open the file.
func Register(db *sqlite3.Conn) {
	RegisterFS(db, util.OSFS{})
}

// RegisterFS registers the CSV virtual table.
// If a filename is specified, fsys is used to open the file.
func RegisterFS(db *sqlite3.Conn, fsys fs.FS) {
	declare := func(db *sqlite3.Conn, _, _, _ string, arg ...string) (_ *table, err error) {
		var (
			filename string
			data     string
			schema   string
			header   bool
			columns  int  = -1
			comma    rune = ','

			done = map[string]struct{}{}
		)

		for _, arg := range arg {
			key, val := getParam(arg)
			if _, ok := done[key]; ok {
				return nil, fmt.Errorf("csv: more than one %q parameter", key)
			}
			switch key {
			case "filename":
				filename = unquoteParam(val)
			case "data":
				data = unquoteParam(val)
			case "schema":
				schema = unquoteParam(val)
			case "header":
				header, err = boolParam(key, val)
			case "columns":
				columns, err = uintParam(key, val)
			case "comma":
				comma, err = runeParam(key, val)
			default:
				return nil, fmt.Errorf("csv: unknown %q parameter", key)
			}
			if err != nil {
				return nil, err
			}
			done[key] = struct{}{}
		}

		if (filename == "") == (data == "") {
			return nil, fmt.Errorf(`csv: must specify either "filename" or "data" but not both`)
		}

		table := &table{
			fsys:   fsys,
			name:   filename,
			data:   data,
			comma:  comma,
			header: header,
		}

		if schema == "" {
			var row []string
			if header || columns < 0 {
				csv, close, err := table.newReader()
				defer close.Close()
				if err != nil {
					return nil, err
				}
				row, err = csv.Read()
				if err != nil {
					return nil, err
				}
			}
			schema = getSchema(header, columns, row)
		}

		err = db.DeclareVtab(schema)
		if err != nil {
			return nil, err
		}
		err = db.VtabConfig(sqlite3.VTAB_DIRECTONLY)
		if err != nil {
			return nil, err
		}
		return table, nil
	}

	sqlite3.CreateModule(db, "csv", declare, declare)
}

type table struct {
	fsys   fs.FS
	name   string
	data   string
	comma  rune
	header bool
}

func (t *table) BestIndex(idx *sqlite3.IndexInfo) error {
	idx.EstimatedCost = 1e6
	return nil
}

func (t *table) Open() (sqlite3.VTabCursor, error) {
	return &cursor{table: t}, nil
}

func (t *table) Rename(new string) error {
	return nil
}

func (t *table) Integrity(schema, table string, flags int) error {
	if flags&1 != 0 {
		return nil
	}
	csv, close, err := t.newReader()
	if err != nil {
		return err
	}
	if close != nil {
		defer close.Close()
	}
	_, err = csv.ReadAll()
	return err
}

func (t *table) newReader() (*csv.Reader, io.Closer, error) {
	var r io.Reader
	var c io.Closer
	if t.name != "" {
		f, err := t.fsys.Open(t.name)
		if err != nil {
			return nil, f, err
		}

		buf := bufio.NewReader(f)
		bom, err := buf.Peek(3)
		if err != nil {
			return nil, f, err
		}
		if string(bom) == "\xEF\xBB\xBF" {
			buf.Discard(3)
		}

		r = buf
		c = f
	} else {
		r = strings.NewReader(t.data)
		c = io.NopCloser(r)
	}

	csv := csv.NewReader(r)
	csv.ReuseRecord = true
	csv.Comma = t.comma
	return csv, c, nil
}

type cursor struct {
	table *table
	close io.Closer
	csv   *csv.Reader
	row   []string
	rowID int64
}

func (c *cursor) Close() error {
	return c.close.Close()
}

func (c *cursor) Filter(idxNum int, idxStr string, arg ...sqlite3.Value) error {
	var err error
	c.csv, c.close, err = c.table.newReader()
	if err != nil {
		return err
	}
	if c.table.header {
		c.Next() // skip header
	}
	c.rowID = 0
	return c.Next()
}

func (c *cursor) Next() (err error) {
	c.rowID++
	c.row, err = c.csv.Read()
	if err != io.EOF {
		return err
	}
	return nil
}

func (c *cursor) EOF() bool {
	return c.row == nil
}

func (c *cursor) RowID() (int64, error) {
	return c.rowID, nil
}

func (c *cursor) Column(ctx *sqlite3.Context, col int) error {
	if col < len(c.row) {
		ctx.ResultText(c.row[col])
	}
	return nil
}
