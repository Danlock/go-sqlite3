package fileio

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/internal/util"
)

func writefile(ctx sqlite3.Context, arg ...sqlite3.Value) {
	if len(arg) < 2 || len(arg) > 4 {
		ctx.ResultError(util.ErrorString("writefile: wrong number of arguments"))
		return
	}

	file := arg[0].Text()

	var mode fs.FileMode
	if len(arg) > 2 {
		mode = fixMode(fs.FileMode(arg[2].Int()))
	}

	n, err := createFileAndDir(file, mode, arg[1])
	if err != nil {
		if len(arg) > 2 {
			ctx.ResultError(fmt.Errorf("writefile: %w", err))
		}
		return
	}

	if mode&fs.ModeSymlink == 0 {
		if len(arg) > 2 {
			err := os.Chmod(file, mode.Perm())
			if err != nil {
				ctx.ResultError(fmt.Errorf("writefile: %w", err))
				return
			}
		}

		if len(arg) > 3 {
			mtime := arg[3].Time(sqlite3.TimeFormatUnixFrac)
			err := os.Chtimes(file, time.Time{}, mtime)
			if err != nil {
				ctx.ResultError(fmt.Errorf("writefile: %w", err))
				return
			}
		}
	}

	if mode.IsRegular() {
		ctx.ResultInt(n)
	}
}

func createFileAndDir(path string, mode fs.FileMode, data sqlite3.Value) (int, error) {
	n, err := createFile(path, mode, data)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0777); err == nil {
			return createFile(path, mode, data)
		}
	}
	return n, err
}

func createFile(path string, mode fs.FileMode, data sqlite3.Value) (int, error) {
	if mode.IsRegular() {
		blob := data.RawBlob()
		return len(blob), os.WriteFile(path, blob, fixPerm(mode, 0666))
	}
	if mode.IsDir() {
		err := os.Mkdir(path, fixPerm(mode, 0777))
		if errors.Is(err, fs.ErrExist) {
			s, err := os.Lstat(path)
			if err == nil && s.IsDir() {
				return 0, nil
			}
		}
		return 0, err
	}
	if mode&fs.ModeSymlink != 0 {
		return 0, os.Symlink(data.Text(), path)
	}
	return 0, fmt.Errorf("invalid mode: %v", mode)
}

func fixMode(mode fs.FileMode) fs.FileMode {
	const (
		S_IFMT   fs.FileMode = 0170000
		S_IFIFO  fs.FileMode = 0010000
		S_IFCHR  fs.FileMode = 0020000
		S_IFDIR  fs.FileMode = 0040000
		S_IFBLK  fs.FileMode = 0060000
		S_IFREG  fs.FileMode = 0100000
		S_IFLNK  fs.FileMode = 0120000
		S_IFSOCK fs.FileMode = 0140000
	)

	switch mode & S_IFMT {
	case S_IFDIR:
		mode |= fs.ModeDir
	case S_IFLNK:
		mode |= fs.ModeSymlink
	case S_IFBLK:
		mode |= fs.ModeDevice
	case S_IFCHR:
		mode |= fs.ModeCharDevice | fs.ModeDevice
	case S_IFIFO:
		mode |= fs.ModeNamedPipe
	case S_IFSOCK:
		mode |= fs.ModeSocket
	case S_IFREG, 0:
		//
	default:
		mode |= fs.ModeIrregular
	}

	return mode &^ S_IFMT
}

func fixPerm(mode fs.FileMode, def fs.FileMode) fs.FileMode {
	if mode.Perm() == 0 {
		return def
	}
	return mode.Perm()
}
