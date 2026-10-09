package safefile

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPublicationFaultsPreserveWholeBytes(t *testing.T) {
	old, next := []byte("old complete uncommitted source"), []byte("new complete source")
	fault := errors.New("injected I/O fault")
	for _, stage := range []string{"write", "before", "after"} {
		t.Run(stage, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "page.md")
			if err := os.WriteFile(path, old, 0o640); err != nil {
				t.Fatal(err)
			}
			h := &hooks{}
			switch stage {
			case "write":
				h.write = func(f *os.File, b []byte) error {
					_, err := f.Write(b[:4])
					if err != nil {
						return err
					}
					return fault
				}
			case "before":
				h.beforePublish = func() error { return fault }
			case "after":
				h.afterPublish = func() error { return fault }
			}
			err := publish(path, next, 0o644, false, h)
			if !errors.Is(err, fault) {
				t.Fatalf("fault not returned: %v", err)
			}
			got, _ := os.ReadFile(path)
			want := old
			if stage == "after" {
				want = next
			}
			if !bytes.Equal(got, want) || Published(err) != (stage == "after") {
				t.Fatalf("wrong publication: %q %v", got, err)
			}
			st, _ := os.Stat(path)
			if st.Mode().Perm() != 0o640 {
				t.Fatalf("mode changed: %v", st.Mode())
			}
			temps, _ := filepath.Glob(filepath.Join(filepath.Dir(path), TempPrefix+"*"))
			if len(temps) != 0 {
				t.Fatalf("scratch remains: %v", temps)
			}
		})
	}
}

func TestExclusiveCreateAndMove(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst.md")
	src := filepath.Join(dir, "src.md")
	if err := CreateFile(dst, []byte("archive-original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CreateFile(dst, []byte("clobber"), 0o644); !errors.Is(err, fs.ErrExist) {
		t.Fatal(err)
	}
	if err := WriteFile(src, []byte("source"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := MoveNoReplace(src, dst); !errors.Is(err, fs.ErrExist) {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dst)
	before, _ := os.ReadFile(src)
	if string(got) != "archive-original" || string(before) != "source" {
		t.Fatal("source/destination changed")
	}
	other := filepath.Join(dir, "other.md")
	if err := MoveNoReplace(src, other); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source still exists")
	}
}

func TestReadersObserveCompleteVersions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "page.md")
	a := bytes.Repeat([]byte("a"), 65536)
	b := bytes.Repeat([]byte("b"), 65536)
	if err := WriteFile(path, a, 0o644); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	done := make(chan struct{})
	bad := make(chan string, 1)
	wg.Go(func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			got, err := os.ReadFile(path)
			if err != nil || (!bytes.Equal(got, a) && !bytes.Equal(got, b)) {
				select {
				case bad <- "torn read":
				default:
				}
				return
			}
		}
	})
	for i := 0; i < 30; i++ {
		data := a
		if i%2 == 0 {
			data = b
		}
		if err := WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	close(done)
	wg.Wait()
	select {
	case s := <-bad:
		t.Fatal(s)
	default:
	}
}

func TestSymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "original")
	link := filepath.Join(dir, "link")
	os.WriteFile(target, []byte("keep"), 0o600)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(link, []byte("lost"), 0o644); err == nil {
		t.Fatal("accepted symlink")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "keep" {
		t.Fatal("target changed")
	}
}

func TestPublicationCrashHelper(t *testing.T) {
	stage := os.Getenv("CANOPY_PUBLICATION_HELPER")
	if stage == "" {
		return
	}
	h := &hooks{}
	block := func() error {
		fmt.Println("publication-boundary")
		for {
			time.Sleep(time.Hour)
		}
	}
	if stage == "before" {
		h.beforePublish = block
	} else {
		h.afterPublish = block
	}
	if err := publish(os.Getenv("CANOPY_PUBLICATION_PATH"), []byte("complete next revision"), 0o644, false, h); err != nil {
		t.Fatal(err)
	}
}
func TestKilledPublicationLeavesWholeOldOrNewFile(t *testing.T) {
	for _, stage := range []string{"before", "after"} {
		t.Run(stage, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "page.md")
			os.WriteFile(path, []byte("complete old revision"), 0o644)
			cmd := exec.Command(os.Args[0], "-test.run=^TestPublicationCrashHelper$")
			cmd.Env = append(os.Environ(), "CANOPY_PUBLICATION_HELPER="+stage, "CANOPY_PUBLICATION_PATH="+path)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
			ready := make(chan string, 1)
			go func() {
				scanner := bufio.NewScanner(stdout)
				if scanner.Scan() {
					ready <- scanner.Text()
				} else {
					ready <- ""
				}
			}()
			select {
			case line := <-ready:
				if line != "publication-boundary" {
					t.Fatal(line)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("child did not reach publication boundary")
			}
			cmd.Process.Kill()
			cmd.Wait()
			got, _ := os.ReadFile(path)
			want := "complete old revision"
			if stage == "after" {
				want = "complete next revision"
			}
			if string(got) != want {
				t.Fatalf("killed publication tore source: %q", got)
			}
			temps, _ := filepath.Glob(filepath.Join(filepath.Dir(path), TempPrefix+"*"))
			for _, temp := range temps {
				if strings.HasSuffix(filepath.Base(temp), ".md") || !strings.HasPrefix(filepath.Base(temp), TempPrefix) {
					t.Fatal("scratch masquerades as a page")
				}
			}
			if err := CleanTemps(filepath.Dir(path)); err != nil {
				t.Fatal(err)
			}
			temps, _ = filepath.Glob(filepath.Join(filepath.Dir(path), TempPrefix+"*"))
			if len(temps) != 0 {
				t.Fatal("abandoned scratch not reclaimed")
			}
		})
	}
}

func TestScratchCleanupPreservesMarkdownSymlinksAndGitInternals(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, ".git"), 0o755)
	keep := []string{TempPrefix + "source.md", filepath.Join(".git", TempPrefix+"internal")}
	for _, name := range keep {
		os.WriteFile(filepath.Join(root, name), []byte("preserve"), 0o600)
	}
	target := filepath.Join(t.TempDir(), "outside")
	os.WriteFile(target, []byte("preserve"), 0o600)
	link := filepath.Join(root, TempPrefix+"symlink")
	os.Symlink(target, link)
	stale := filepath.Join(root, TempPrefix+"12345")
	os.WriteFile(stale, []byte("stale scratch"), 0o600)
	if err := CleanTemps(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale scratch remains")
	}
	for _, name := range keep {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(data) != "preserve" {
			t.Fatalf("original removed: %s", name)
		}
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatal("followed or removed symlink")
	}
}

func TestScratchCleanupThroughRootAlias(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "wiki-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, TempPrefix+"interrupted")
	if err := os.WriteFile(stale, []byte("incomplete scratch"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CleanTemps(alias); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("successful alias cleanup left abandoned scratch")
	}
}
