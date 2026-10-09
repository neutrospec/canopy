package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/checkout"
	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/reads"
	"github.com/neutrospec/canopy/internal/reconcile"
	"github.com/neutrospec/canopy/internal/wiki"
	"github.com/spf13/cobra"
)

func mutationWiki(t *testing.T) *config.Wiki {
	t.Helper()
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	for _, dir := range w.Cfg.Schema.PageDirs {
		if err := os.MkdirAll(filepath.Join(w.Root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"alpha": "baseline source [[alpha#self]]", "omega": "Inbound [[concepts/alpha.md#here|display]] and `[[alpha]]`."} {
		if err := os.WriteFile(filepath.Join(w.Root, "concepts", name+".md"), []byte(wiki.NewPageContent(name, "concept", "2026-01-01", nil, nil, body)), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	return w
}
func mutationGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func mutationRemote(t *testing.T, w *config.Wiki) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "origin.git")
	mutationGit(t, w.Root, "init", "-b", "main")
	mutationGit(t, w.Root, "config", "user.name", "fixture")
	mutationGit(t, w.Root, "config", "user.email", "fixture@example.invalid")
	mutationGit(t, w.Root, "init", "--bare", remote)
	mutationGit(t, w.Root, "remote", "add", "origin", remote)
	mutationGit(t, w.Root, "add", ".")
	mutationGit(t, w.Root, "commit", "-m", "baseline")
	mutationGit(t, w.Root, "push", "-u", "origin", "main")
	return remote
}
func mutationRun(t *testing.T, w *config.Wiki, c *cobra.Command, args ...string) (map[string]any, error) {
	t.Helper()
	oldWiki, oldJSON := flagWiki, flagJSON
	flagWiki = w.Root
	flagJSON = true
	defer func() { flagWiki, flagJSON = oldWiki, oldJSON }()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldOut := os.Stdout
	os.Stdout = write
	done := make(chan []byte, 1)
	go func() { b, _ := io.ReadAll(read); done <- b }()
	c.SetArgs(args)
	c.SilenceErrors = true
	c.SilenceUsage = true
	commandErr := c.Execute()
	write.Close()
	os.Stdout = oldOut
	out := <-done
	read.Close()
	var result map[string]any
	decoder := json.NewDecoder(bytes.NewReader(out))
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("stdout not a result object: %v\n%s (command %v)", err, out, commandErr)
	}
	var second any
	if err := decoder.Decode(&second); err != io.EOF {
		t.Fatalf("stdout has multiple values or trailing data: %v\n%s", err, out)
	}
	return result, commandErr
}
func mutationVerb(t *testing.T, w *config.Wiki, verb string) (*cobra.Command, []string, string) {
	t.Helper()
	switch verb {
	case "new":
		return cmdNew(), []string{"Beta", "--type", "concept", "--slug", "beta"}, "concepts/beta.md"
	case "update":
		path := filepath.Join(t.TempDir(), "body")
		os.WriteFile(path, []byte("replacement source"), 0o600)
		return cmdUpdate(), []string{"alpha", "--body-file", path}, "concepts/alpha.md"
	case "mv":
		return cmdMv(), []string{"alpha", "--slug", "beta", "--type", "entity"}, "entities/beta.md"
	case "archive":
		return cmdArchive(), []string{"alpha"}, "_archive/concepts/alpha.md"
	case "rm":
		return cmdRm(), []string{"alpha", "--force"}, ""
	case "checkin":
		scan, err := wiki.Scan(w)
		if err != nil {
			t.Fatal(err)
		}
		info, err := checkout.Checkout(w, scan, "alpha")
		if err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(info.Path)
		os.WriteFile(info.Path, []byte(strings.Replace(string(data), "baseline source", "checked in source", 1)), 0o600)
		return cmdCheckin(), []string{"alpha"}, "concepts/alpha.md"
	default:
		t.Fatal(verb)
		return nil, nil, ""
	}
}
func assertMutationApplied(t *testing.T, w *config.Wiki, result map[string]any, verb, path string) {
	t.Helper()
	if result["command"] != verb || result["applied"] != "applied" {
		t.Fatalf("wrong command/application: %+v", result)
	}
	payload := result["result"].(map[string]any)
	if path != "" {
		data, err := os.ReadFile(filepath.Join(w.Root, path))
		if err != nil {
			t.Fatal(err)
		}
		if payload["applied_hash"] != reconcile.HashBytes(data) {
			t.Fatalf("reported applied hash differs from actual bytes: %+v", payload)
		}
	} else if _, err := os.Stat(filepath.Join(w.Root, "concepts/alpha.md")); !os.IsNotExist(err) {
		t.Fatal("deleted page remains")
	}
}

func TestSixMutationVerbsSingleObjectBasicAndSync(t *testing.T) {
	for _, verb := range []string{"new", "update", "mv", "archive", "rm", "checkin"} {
		for _, syncNow := range []bool{false, true} {
			name := verb + "/basic"
			if syncNow {
				name = verb + "/sync"
			}
			t.Run(name, func(t *testing.T) {
				w := mutationWiki(t)
				remote := ""
				if syncNow {
					remote = mutationRemote(t, w)
				}
				c, args, path := mutationVerb(t, w, verb)
				if syncNow {
					args = append(args, "--sync")
				}
				result, err := mutationRun(t, w, c, args...)
				if err != nil {
					t.Fatal(err)
				}
				if result["status"] != "ok" || result["stage"] != "complete" {
					t.Fatalf("wrong success result: %+v", result)
				}
				assertMutationApplied(t, w, result, verb, path)
				if syncNow {
					if result["committed"] != true || result["pushed"] != true {
						t.Fatalf("sync compatibility results missing/wrong: %+v", result)
					}
					if local, remoteTip := mutationGit(t, w.Root, "rev-parse", "HEAD"), mutationGit(t, remote, "rev-parse", "refs/heads/main"); local != remoteTip {
						t.Fatal("reported push not in remote")
					}
				}
				if verb == "mv" {
					st, _ := os.Stat(filepath.Join(w.Root, path))
					if st.Mode().Perm() != 0o640 {
						t.Fatal("move changed source permissions")
					}
				}
			})
		}
	}
}

func TestSixMutationPostpublicationIndexFailureReportsActualAppliedHash(t *testing.T) {
	for _, verb := range []string{"new", "update", "mv", "archive", "rm", "checkin"} {
		for _, syncNow := range []bool{false, true} {
			t.Run(verb+map[bool]string{false: "/basic", true: "/sync"}[syncNow], func(t *testing.T) {
				w := mutationWiki(t)
				c, args, path := mutationVerb(t, w, verb)
				if syncNow {
					args = append(args, "--sync")
				}
				os.Mkdir(w.IndexMDPath(), 0o755)
				result, err := mutationRun(t, w, c, args...)
				if err == nil {
					t.Fatal("index obstruction accepted")
				}
				if result["status"] != "error" || result["stage"] != "index" || result["repair"] == "" {
					t.Fatalf("failed stage hidden: %+v", result)
				}
				assertMutationApplied(t, w, result, verb, path)
				if syncNow {
					sync := result["sync"].(map[string]any)
					if sync["pull"] != "not_attempted" || result["committed"] != false || result["pushed"] != false {
						t.Fatal("sync ran despite postprocessing failure")
					}
				}
				if verb == "checkin" {
					if result["result"].(map[string]any)["cleanup_pending"] != true {
						t.Fatal("retained checkout hidden")
					}
					os.Remove(w.IndexMDPath())
					again, err := mutationRun(t, w, cmdCheckin(), "alpha")
					if err != nil {
						t.Fatal(err)
					}
					if again["result"].(map[string]any)["resumed"] != true {
						t.Fatal("checkin did not resume")
					}
				}
			})
		}
	}
}

func TestMutationRefusalsPreserveOldHashBeforePublication(t *testing.T) {
	for _, verb := range []string{"new", "update", "mv", "archive", "rm", "checkin"} {
		for _, syncNow := range []bool{false, true} {
			t.Run(verb+map[bool]string{false: "/basic", true: "/sync"}[syncNow], func(t *testing.T) {
				w := mutationWiki(t)
				c, args, _ := mutationVerb(t, w, verb)
				switch verb {
				case "new":
					args = []string{"Alpha", "--type", "concept", "--slug", "alpha"}
				case "update":
					data, _ := os.ReadFile(filepath.Join(w.Root, "concepts/alpha.md"))
					os.WriteFile(filepath.Join(w.Root, "concepts/alpha.md"), []byte(strings.Replace(string(data), "updated: 2026-01-01", "updated: |\n  2026-01-01", 1)), 0o640)
				case "mv":
					args = []string{"alpha", "--type", "unsupported"}
				case "archive":
					os.MkdirAll(filepath.Join(w.Root, "_archive/concepts"), 0o755)
					os.WriteFile(filepath.Join(w.Root, "_archive/concepts/alpha.md"), []byte("previous archive"), 0o600)
				case "rm":
					args = []string{"alpha"}
				case "checkin":
					open, _ := checkout.Open(w)
					data, _ := os.ReadFile(open[0].Path)
					os.WriteFile(open[0].Path, []byte(strings.Replace(string(data), "type: concept", "type: entity", 1)), 0o600)
				}
				before, _ := os.ReadFile(filepath.Join(w.Root, "concepts/alpha.md"))
				if syncNow {
					args = append(args, "--sync")
				}
				result, err := mutationRun(t, w, c, args...)
				if err == nil || result["status"] != "error" || result["applied"] != "none" {
					t.Fatalf("refusal falsely applied: %+v %v", result, err)
				}
				after, _ := os.ReadFile(filepath.Join(w.Root, "concepts/alpha.md"))
				if reconcile.HashBytes(before) != reconcile.HashBytes(after) {
					t.Fatal("prepublication old source hash changed")
				}
				if syncNow && result["committed"] != false {
					t.Fatal("committed refused mutation")
				}
			})
		}
	}
}

func TestSixMutationCommitAndPushFailuresRemainOneObject(t *testing.T) {
	for _, verb := range []string{"new", "update", "mv", "archive", "rm", "checkin"} {
		for _, stage := range []string{"commit", "push"} {
			t.Run(verb+"/"+stage, func(t *testing.T) {
				w := mutationWiki(t)
				remote := mutationRemote(t, w)
				oldTip := mutationGit(t, remote, "rev-parse", "refs/heads/main")
				c, args, path := mutationVerb(t, w, verb)
				args = append(args, "--sync")
				hook := filepath.Join(w.Root, ".git/hooks/pre-commit")
				if stage == "push" {
					hook = filepath.Join(remote, "hooks/pre-receive")
				}
				os.WriteFile(hook, []byte("#!/bin/sh\necho fixture rejection >&2\nexit 1\n"), 0o755)
				result, err := mutationRun(t, w, c, args...)
				if err == nil || result["stage"] != stage || result["status"] != "error" {
					t.Fatalf("wrong sync failure: %+v %v", result, err)
				}
				assertMutationApplied(t, w, result, verb, path)
				if result["committed"] != (stage == "push") || result["pushed"] != false {
					t.Fatalf("actual Git results hidden: %+v", result)
				}
				if tip := mutationGit(t, remote, "rev-parse", "refs/heads/main"); tip != oldTip {
					t.Fatal("rejected sync changed remote")
				}
			})
		}
	}
}

func TestCheckinAndMoveNoopStillHonorRequestedSync(t *testing.T) {
	for _, verb := range []string{"checkin", "mv"} {
		for _, syncNow := range []bool{false, true} {
			t.Run(verb+map[bool]string{false: "/basic", true: "/sync"}[syncNow], func(t *testing.T) {
				w := mutationWiki(t)
				if syncNow {
					mutationRemote(t, w)
				}
				before, _ := os.ReadFile(filepath.Join(w.Root, "concepts/alpha.md"))
				var c *cobra.Command
				var args []string
				if verb == "checkin" {
					scan, _ := wiki.Scan(w)
					if _, err := checkout.Checkout(w, scan, "alpha"); err != nil {
						t.Fatal(err)
					}
					c = cmdCheckin()
					args = []string{"alpha"}
				} else {
					c = cmdMv()
					args = []string{"alpha", "--slug", "alpha"}
				}
				if syncNow {
					args = append(args, "--sync")
				}
				result, err := mutationRun(t, w, c, args...)
				if err != nil || result["status"] != "noop" || result["applied"] != "none" {
					t.Fatalf("wrong noop result: %+v %v", result, err)
				}
				after, _ := os.ReadFile(filepath.Join(w.Root, "concepts/alpha.md"))
				if reconcile.HashBytes(before) != reconcile.HashBytes(after) {
					t.Fatal("noop changed page")
				}
				if syncNow && result["pushed"] != true {
					t.Fatal("explicit sync skipped on noop")
				}
			})
		}
	}
}

func TestLegacyTagShapesRemainToleratedByUpdate(t *testing.T) {
	for _, shape := range []string{"missing", "null", "scalar", "legacy-unknown"} {
		t.Run(shape, func(t *testing.T) {
			w := mutationWiki(t)
			path := filepath.Join(w.Root, "concepts/alpha.md")
			data, _ := os.ReadFile(path)
			tags := ""
			switch shape {
			case "null":
				tags = "tags: null\n"
			case "scalar":
				tags = "tags: tool\n"
			case "legacy-unknown":
				tags = "tags: legacy-unknown\n"
			}
			data = []byte(strings.Replace(string(data), "tags: []\n", tags, 1))
			os.WriteFile(path, data, 0o640)
			result, err := mutationRun(t, w, cmdUpdate(), "alpha")
			if err != nil || result["status"] != "ok" {
				t.Fatalf("lint diagnostic became hard gate: %+v %v", result, err)
			}
			after, _ := os.ReadFile(path)
			if tags != "" && !strings.Contains(string(after), tags) {
				t.Fatal("legacy tags rewritten")
			}
			if tags == "" && strings.Contains(string(after), "tags:") {
				t.Fatal("missing legacy key silently inserted")
			}
		})
	}
}

func TestMoveKnownStateFailureRefusesBeforePublication(t *testing.T) {
	w := mutationWiki(t)
	path := reads.Path(w)
	os.MkdirAll(filepath.Dir(path), 0o755)
	broken := []byte("{broken history")
	os.WriteFile(path, broken, 0o600)
	before, _ := os.ReadFile(filepath.Join(w.Root, "concepts/alpha.md"))
	result, err := mutationRun(t, w, cmdMv(), "alpha", "--slug", "beta")
	if err == nil || result["stage"] != "state" || result["applied"] != "none" {
		t.Fatalf("known state failure was not preflighted: %+v %v", result, err)
	}
	data, _ := os.ReadFile(filepath.Join(w.Root, "concepts/alpha.md"))
	if reconcile.HashBytes(before) != reconcile.HashBytes(data) {
		t.Fatal("preflight changed original source")
	}
	after, _ := os.ReadFile(path)
	if reconcile.HashBytes(broken) != reconcile.HashBytes(after) {
		t.Fatal("broken history overwritten")
	}
}

func TestMoveUnexpectedStateSaveFailureReportsAppliedPage(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires filesystem permission enforcement for this UID")
	}
	w := mutationWiki(t)
	s, err := reads.Load(w)
	if err != nil {
		t.Fatal(err)
	}
	s.MarkAgent("alpha", time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC))
	if err := s.Save(w); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(reads.AgentPath(w))
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	result, err := mutationRun(t, w, cmdMv(), "alpha", "--slug", "beta")
	if err == nil || result["stage"] != "state" || result["applied"] != "applied" {
		t.Fatalf("unexpected save failure hid application: %+v %v", result, err)
	}
	data, _ := os.ReadFile(filepath.Join(w.Root, "concepts/beta.md"))
	if result["result"].(map[string]any)["applied_hash"] != reconcile.HashBytes(data) {
		t.Fatal("wrong applied hash after state I/O failure")
	}
}

func TestAmbiguousPushAcknowledgementReportsObservedRemote(t *testing.T) {
	for _, observable := range []bool{true, false} {
		t.Run(map[bool]string{true: "accepted-observed", false: "unknown"}[observable], func(t *testing.T) {
			w := mutationWiki(t)
			remote := mutationRemote(t, w)
			realGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			wrapper := filepath.Join(dir, "git")
			// Quote only test-owned executable paths; shell arguments pass through.
			quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
			body := "#!/bin/sh\ncase \"$1\" in\npush) "
			if observable {
				body += quote(realGit) + " \"$@\" || exit $?\n"
			}
			body += "echo simulated lost acknowledgement >&2\nexit 1;;\n"
			if !observable {
				body += "ls-remote) exit 2;;\n"
			}
			body += "esac\nexec " + quote(realGit) + " \"$@\"\n"
			os.WriteFile(wrapper, []byte(body), 0o755)
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			result, err := mutationRun(t, w, cmdNew(), "Beta", "--type", "concept", "--slug", "beta", "--sync")
			if err == nil || result["stage"] != "push" || result["committed"] != true {
				t.Fatalf("wrong failure state: %+v %v", result, err)
			}
			sync := result["sync"].(map[string]any)
			if observable {
				if result["pushed"] != true || sync["push"] != "succeeded" {
					t.Fatalf("confirmed remote effect hidden: %+v", result)
				}
				if local, tip := mutationGit(t, w.Root, "rev-parse", "HEAD"), mutationGit(t, remote, "rev-parse", "refs/heads/main"); local != tip {
					t.Fatal("remote verification untrue")
				}
			} else if result["pushed"] != false || sync["push"] != "unknown" {
				t.Fatalf("unobservable failure falsely known: %+v", result)
			}
		})
	}
}

func TestMoveBacklinkFailureReportsPartialAndPreservesSources(t *testing.T) {
	w := mutationWiki(t)
	source := filepath.Join(w.Root, "concepts/omega.md")
	bytes, _ := os.ReadFile(source)
	external := filepath.Join(t.TempDir(), "external.md")
	os.WriteFile(external, bytes, 0o600)
	os.Remove(source)
	os.Symlink(external, source)
	result, err := mutationRun(t, w, cmdMv(), "alpha", "--slug", "beta")
	if err == nil || result["applied"] != "partial" || result["stage"] != "backlinks" {
		t.Fatalf("partial move hidden: %+v %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(w.Root, "concepts/beta.md")); err != nil {
		t.Fatal("moved source missing")
	}
	got, _ := os.ReadFile(external)
	if reconcile.HashBytes(bytes) != reconcile.HashBytes(got) {
		t.Fatal("backlink failure changed external source")
	}
}
