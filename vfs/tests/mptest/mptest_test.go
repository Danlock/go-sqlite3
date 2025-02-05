package mptest

import (
	"bytes"
	"compress/bzip2"
	"context"
	"crypto/rand"
	"embed"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ncruces/go-sqlite3/internal/util"
	"github.com/ncruces/go-sqlite3/vfs"
	_ "github.com/ncruces/go-sqlite3/vfs/memdb"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed testdata/mptest.wasm.bz2
var compressed string

//go:embed testdata/*.*test
var scripts embed.FS

var (
	rt        wazero.Runtime
	module    wazero.CompiledModule
	instances atomic.Uint64
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	rt = wazero.NewRuntime(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	env := vfs.ExportHostFunctions(rt.NewHostModuleBuilder("env"))
	env.NewFunctionBuilder().WithFunc(system).Export("system")
	_, err := env.Instantiate(ctx)
	if err != nil {
		panic(err)
	}

	binary, err := io.ReadAll(bzip2.NewReader(strings.NewReader(compressed)))
	if err != nil {
		panic(err)
	}

	module, err = rt.CompileModule(ctx, binary)
	if err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func config(ctx context.Context) wazero.ModuleConfig {
	name := strconv.FormatUint(instances.Add(1), 10)
	log := ctx.Value(logger{}).(io.Writer)
	fs, err := fs.Sub(scripts, "testdata")
	if err != nil {
		panic(err)
	}

	return wazero.NewModuleConfig().
		WithName(name).WithStdout(log).WithStderr(log).WithFS(fs).
		WithSysWalltime().WithSysNanotime().WithSysNanosleep().
		WithOsyield(runtime.Gosched).
		WithRandSource(rand.Reader)
}

func system(ctx context.Context, mod api.Module, ptr uint32) uint32 {
	buf, _ := mod.Memory().Read(ptr, mod.Memory().Size()-ptr)
	buf = buf[:bytes.IndexByte(buf, 0)]

	args := strings.Split(string(buf), " ")
	for i := range args {
		args[i] = strings.Trim(args[i], `"`)
	}
	args = args[:len(args)-1]

	cfg := config(ctx).WithArgs(args...)
	go func() {
		ctx := util.NewContext(ctx)
		mod, _ := rt.InstantiateModule(ctx, module, cfg)
		mod.Close(ctx)
	}()
	return 0
}

func Test_config01(t *testing.T) {
	ctx := util.NewContext(newContext(t))
	name := filepath.Join(t.TempDir(), "test.db")
	cfg := config(ctx).WithArgs("mptest", name, "config01.test")
	mod, err := rt.InstantiateModule(ctx, module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	mod.Close(ctx)
}

func Test_config02(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	if os.Getenv("CI") != "" {
		t.Skip("skipping in CI")
	}

	ctx := util.NewContext(newContext(t))
	name := filepath.Join(t.TempDir(), "test.db")
	cfg := config(ctx).WithArgs("mptest", name, "config02.test")
	mod, err := rt.InstantiateModule(ctx, module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	mod.Close(ctx)
}

func Test_crash01(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	ctx := util.NewContext(newContext(t))
	name := filepath.Join(t.TempDir(), "test.db")
	cfg := config(ctx).WithArgs("mptest", name, "crash01.test")
	mod, err := rt.InstantiateModule(ctx, module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	mod.Close(ctx)
}

func Test_multiwrite01(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	ctx := util.NewContext(newContext(t))
	name := filepath.Join(t.TempDir(), "test.db")
	cfg := config(ctx).WithArgs("mptest", name, "multiwrite01.test")
	mod, err := rt.InstantiateModule(ctx, module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	mod.Close(ctx)
}

func Test_config01_memory(t *testing.T) {
	ctx := util.NewContext(newContext(t))
	cfg := config(ctx).WithArgs("mptest", "/test.db",
		"config01.test",
		"--vfs", "memdb",
		"--timeout", "1000")
	mod, err := rt.InstantiateModule(ctx, module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	mod.Close(ctx)
}

func Test_multiwrite01_memory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	ctx := util.NewContext(newContext(t))
	cfg := config(ctx).WithArgs("mptest", "/test.db",
		"multiwrite01.test",
		"--vfs", "memdb",
		"--timeout", "1000")
	mod, err := rt.InstantiateModule(ctx, module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	mod.Close(ctx)
}

func newContext(t *testing.T) context.Context {
	return context.WithValue(context.Background(), logger{}, &testWriter{T: t})
}

type logger struct{}

type testWriter struct {
	// +checklocks:mtx
	*testing.T
	// +checklocks:mtx
	buf []byte
	mtx sync.Mutex
}

func (l *testWriter) Write(p []byte) (n int, err error) {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	l.buf = append(l.buf, p...)
	for {
		before, after, found := bytes.Cut(l.buf, []byte("\n"))
		if !found {
			return len(p), nil
		}
		l.Logf("%s", before)
		l.buf = after
	}
}
