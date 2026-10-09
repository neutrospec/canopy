package wikilock

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/safefile"
)

func TestAliasAndSharedLease(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	g, err := Acquire(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if other, err := Acquire(ctx, alias); err == nil {
		other.Unlock()
		t.Fatal("alias bypassed exclusive lock")
	}
	g.Unlock()
	a, err := AcquireShared(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Unlock()
	b, err := AcquireShared(context.Background(), alias)
	if err != nil {
		t.Fatal(err)
	}
	b.Unlock()
}

func TestLockProcessHelper(t *testing.T) {
	if os.Getenv("CANOPY_LOCK_HELPER") != "1" {
		return
	}
	g, err := Acquire(context.Background(), os.Getenv("CANOPY_LOCK_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Unlock()
	fmt.Println("acquired")
	for {
		time.Sleep(time.Hour)
	}
}

func TestRMWProcessHelper(t *testing.T) {
	if os.Getenv("CANOPY_RMW_HELPER") != "1" {
		return
	}
	root := os.Getenv("CANOPY_LOCK_ROOT")
	path := filepath.Join(root, "counter")
	for range 20 {
		g, err := Acquire(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			g.Unlock()
			t.Fatal(err)
		}
		n, err := strconv.Atoi(string(data))
		if err != nil {
			g.Unlock()
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
		err = safefile.WriteFile(path, []byte(strconv.Itoa(n+1)), 0o600)
		g.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	}
}
func TestAliasMultiprocessRMWSumsAllAcceptedWrites(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "counter"), []byte("0"), 0o600)
	var children []*exec.Cmd
	for _, path := range []string{root, alias, root, alias} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestRMWProcessHelper$")
		cmd.Env = append(os.Environ(), "CANOPY_RMW_HELPER=1", "CANOPY_LOCK_ROOT="+path)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		children = append(children, cmd)
	}
	for _, cmd := range children {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := os.ReadFile(filepath.Join(root, "counter"))
	if string(got) != "80" {
		t.Fatalf("accepted writes lost: %s", got)
	}
}

func TestMultiprocessAndCrashRelease(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockProcessHelper$")
	cmd.Env = append(os.Environ(), "CANOPY_LOCK_HELPER=1", "CANOPY_LOCK_ROOT="+root)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	ready := make(chan error, 1)
	go func() { b := make([]byte, len("acquired\n")); _, err := stdout.Read(b); ready <- err }()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child did not acquire")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	if g, err := Acquire(ctx, root); err == nil {
		g.Unlock()
		t.Fatal("process bypassed lock")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	cmd.Wait()
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	g, err := Acquire(ctx2, root)
	if err != nil {
		t.Fatalf("dead process retained lock: %v", err)
	}
	g.Unlock()
}
