#!/usr/bin/env python3
"""Disposable real-lite-binary acceptance for PLAN-03/04/05/06/08.

Run from any directory. All Go/Git/canopy children receive disposable HOME/XDG/
TMPDIR before startup. Local bare repositories are the only push destinations.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time

REPO = Path(__file__).resolve().parents[4]
OUTPUT = Path(__file__).with_name("preservation-cli-results.json")


def digest(data):
    return hashlib.sha256(data).hexdigest()


def main():
    scratch = Path(tempfile.mkdtemp(prefix="canopy-preservation-binary-"))
    env = os.environ.copy()
    for key, sub in {"HOME": "home", "XDG_CONFIG_HOME": "config",
                     "XDG_CACHE_HOME": "cache", "XDG_DATA_HOME": "data",
                     "XDG_STATE_HOME": "state", "TMPDIR": "tmp"}.items():
        path = scratch / sub
        path.mkdir()
        env[key] = str(path)
    env.update(GOTELEMETRY="off", GOPROXY="off", GOSUMDB="off",
               GOMODCACHE="/Users/nobocop/go/pkg/mod", CGO_ENABLED="0",
               GIT_CONFIG_GLOBAL=os.devnull, GIT_CONFIG_NOSYSTEM="1")
    env.pop("CANOPY_WIKI", None)
    report = {"scratch": str(scratch), "isolation": {key: env[key] for key in
              ("HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME",
               "XDG_STATE_HOME", "TMPDIR", "GOTELEMETRY", "GOPROXY", "GOMODCACHE")},
              "cases": [], "commands": []}
    binary = scratch / "canopy"

    def run(argv, cwd=REPO, check=True):
        p = subprocess.run([str(x) for x in argv], cwd=cwd, env=env,
                           text=True, capture_output=True, timeout=120)
        report["commands"].append({"argv": [str(x) for x in argv], "cwd": str(cwd),
                                   "exit": p.returncode, "stdout": p.stdout, "stderr": p.stderr})
        if check and p.returncode:
            raise AssertionError(f"{argv}: exit {p.returncode}: {p.stderr}")
        return p

    def git(w, *args):
        return run(["git", *args], cwd=w).stdout.strip()

    def page(body="baseline source [[alpha#self]]"):
        return ("---\ntitle: Alpha\ncreated: 2026-01-01\nupdated: 2026-01-01\n"
                "type: concept\ntags: []\nsources: []\n---\n\n" + body + "\n").encode()

    def fixture(name, sync=False):
        root = scratch / name
        root.mkdir()
        for directory in ("concepts", "entities", "comparisons"):
            (root / directory).mkdir()
        (root / "concepts/alpha.md").write_bytes(page())
        (root / "concepts/omega.md").write_bytes(page("Inbound [[concepts/alpha.md#part|alias]] and `[[alpha]]`."))
        remote = None
        if sync:
            remote = scratch / (name + ".git")
            git(root, "init", "-b", "main")
            git(root, "config", "user.name", "fixture")
            git(root, "config", "user.email", "fixture@example.invalid")
            git(root, "init", "--bare", remote)
            git(root, "remote", "add", "origin", remote)
            git(root, "add", ".")
            git(root, "commit", "-m", "baseline")
            git(root, "push", "-u", "origin", "main")
        return root, remote

    def canopy(w, verb, *args, expect=0, mutation=True):
        p = run([binary, "--wiki", w, "--json", verb, *args], check=False)
        if expect == 0:
            assert p.returncode == 0, p.stderr
        else:
            assert p.returncode != 0, "expected failure"
            assert p.stderr, "failed binary omitted diagnostic stderr"
        result = json.loads(p.stdout)  # whole stdout, rejects pretty-object concatenation
        assert isinstance(result, dict)
        if mutation:
            assert result["schema_version"] == 1 and result["command"] == verb
            assert result["status"] == ("error" if expect else result["status"])
        return result

    def operation(w, verb):
        if verb == "new":
            return ["Beta", "--type", "concept", "--slug", "beta"], "concepts/beta.md"
        if verb == "update":
            body = scratch / (w.name + "-body")
            body.write_text("replacement source\n")
            return ["alpha", "--body-file", str(body)], "concepts/alpha.md"
        if verb == "mv":
            return ["alpha", "--slug", "beta", "--type", "entity"], "entities/beta.md"
        if verb == "archive":
            return ["alpha"], "_archive/concepts/alpha.md"
        if verb == "rm":
            return ["alpha", "--force"], None
        info = canopy(w, "checkout", "alpha", mutation=False)
        copy = Path(info["path"])
        copy.write_bytes(copy.read_bytes().replace(b"baseline source", b"checked in source"))
        return ["alpha"], "concepts/alpha.md"

    def applied(w, r, path):
        assert r["applied"] == "applied", r
        if path:
            actual = digest((w / path).read_bytes())
            assert r["result"]["applied_hash"] == actual, (actual, r)
            return actual
        assert not (w / "concepts/alpha.md").exists()
        return None

    def case(name, fn):
        try:
            details = fn() or {}
            report["cases"].append({"name": name, "pass": True, **details})
        except Exception as exc:
            report["cases"].append({"name": name, "pass": False, "error": repr(exc)})

    try:
        report["toolchain"] = run(["go", "version"]).stdout.strip()
        report["revision"] = git(REPO, "rev-parse", "HEAD")
        owned = [REPO / "cmd/canopy/write.go", REPO / "cmd/canopy/checkout.go"]
        for package in ("safefile", "wikilock", "gitops", "checkout", "wiki", "writeops", "genindex", "logops"):
            owned.extend((REPO / "internal" / package).glob("*.go"))
        report["owned_source_sha256"] = {str(p.relative_to(REPO)): digest(p.read_bytes()) for p in sorted(owned)}
        run(["go", "build", "-o", binary, "./cmd/canopy"])
        report["binary_sha256"] = digest(binary.read_bytes())

        for verb in ("new", "update", "mv", "archive", "rm", "checkin"):
            for sync in (False, True):
                suffix = "sync" if sync else "basic"

                def success(verb=verb, sync=sync, suffix=suffix):
                    w, remote = fixture(f"{verb}-success-{suffix}", sync)
                    args, path = operation(w, verb)
                    r = canopy(w, verb, *args, *(["--sync"] if sync else []))
                    assert r["status"] == "ok" and r["stage"] == "complete"
                    actual = applied(w, r, path)
                    if sync:
                        assert r["committed"] and r["pushed"]
                        assert git(w, "rev-parse", "HEAD") == git(remote, "rev-parse", "refs/heads/main")
                    return {"applied_hash": actual, "result": r}
                case(f"{verb}/{suffix}/success", success)

                def refusal(verb=verb, sync=sync, suffix=suffix):
                    w, _ = fixture(f"{verb}-refusal-{suffix}")
                    args, _ = operation(w, verb)
                    if verb == "new":
                        args = ["Alpha", "--type", "concept", "--slug", "alpha"]
                    elif verb == "update":
                        p = w / "concepts/alpha.md"
                        p.write_bytes(p.read_bytes().replace(b"updated: 2026-01-01", b"updated: |\n  2026-01-01"))
                    elif verb == "mv":
                        args = ["alpha", "--type", "unsupported"]
                    elif verb == "archive":
                        p = w / "_archive/concepts/alpha.md"
                        p.parent.mkdir(parents=True)
                        p.write_bytes(b"previous archive source")
                    elif verb == "rm":
                        args = ["alpha"]
                    else:
                        info = canopy(w, "checkout", "--list", mutation=False)["checkouts"][0]
                        p = Path(info["path"])
                        p.write_bytes(p.read_bytes().replace(b"type: concept", b"type: entity"))
                    old = digest((w / "concepts/alpha.md").read_bytes())
                    r = canopy(w, verb, *args, *(["--sync"] if sync else []), expect=1)
                    assert r["applied"] == "none"
                    assert old == digest((w / "concepts/alpha.md").read_bytes())
                    if sync:
                        assert not r["committed"] and not r["pushed"] and r["sync"]["pull"] == "not_attempted"
                    return {"old_hash": old, "after_hash": old, "result": r}
                case(f"{verb}/{suffix}/refusal", refusal)

                def postpublish(verb=verb, sync=sync, suffix=suffix):
                    w, _ = fixture(f"{verb}-index-fault-{suffix}")
                    args, path = operation(w, verb)
                    (w / "index.md").mkdir()
                    r = canopy(w, verb, *args, *(["--sync"] if sync else []), expect=1)
                    assert r["stage"] == "index" and r["repair"]
                    actual = applied(w, r, path)
                    if sync:
                        assert r["sync"]["pull"] == "not_attempted"
                    if verb == "checkin":
                        assert r["result"]["cleanup_pending"]
                        (w / "index.md").rmdir()
                        resumed = canopy(w, "checkin", "alpha")
                        assert resumed["result"]["resumed"]
                        assert digest((w / path).read_bytes()) == actual
                        logs = b"".join(p.read_bytes() for p in (w / "logs").glob("*.jsonl"))
                        assert logs.count(b'"action":"update"') == 1
                    return {"applied_hash": actual, "result": r}
                case(f"{verb}/{suffix}/postpublish-index", postpublish)

            for failure in ("commit", "push"):
                def gitfail(verb=verb, failure=failure):
                    w, remote = fixture(f"{verb}-{failure}-fault", True)
                    old_tip = git(remote, "rev-parse", "refs/heads/main")
                    args, path = operation(w, verb)
                    hook = (w / ".git/hooks/pre-commit") if failure == "commit" else (remote / "hooks/pre-receive")
                    hook.write_text("#!/bin/sh\necho fixture rejection >&2\nexit 1\n")
                    hook.chmod(0o755)
                    r = canopy(w, verb, *args, "--sync", expect=1)
                    assert r["stage"] == failure
                    actual = applied(w, r, path)
                    assert r["committed"] == (failure == "push") and not r["pushed"]
                    assert git(remote, "rev-parse", "refs/heads/main") == old_tip
                    return {"applied_hash": actual, "remote_before": old_tip,
                            "remote_after": old_tip, "result": r}
                case(f"{verb}/sync/{failure}-failure", gitfail)

        def autostash():
            w, remote = fixture("autostash", True)
            peer = scratch / "peer"
            git(w, "clone", remote, peer)
            git(peer, "checkout", "main")
            git(peer, "config", "user.name", "fixture")
            git(peer, "config", "user.email", "fixture@example.invalid")
            p = peer / "concepts/alpha.md"
            p.write_bytes(p.read_bytes().replace(b"baseline source", b"REMOTE-SOURCE"))
            git(peer, "commit", "-am", "remote")
            git(peer, "push")
            remote_tip = git(remote, "rev-parse", "refs/heads/main")
            p = w / "concepts/alpha.md"
            p.write_bytes(p.read_bytes().replace(b"baseline source", b"LOCAL-SOURCE"))
            r = canopy(w, "sync", expect=1, mutation=False)
            assert r["stage"] == "pull" and not r["committed"] and not r["pushed"]
            assert git(remote, "rev-parse", "refs/heads/main") == remote_tip
            assert git(w, "ls-files", "-u")
            assert "LOCAL-SOURCE" in git(w, "show", "stash@{0}:concepts/alpha.md")
            assert "REMOTE-SOURCE" in git(remote, "show", "refs/heads/main:concepts/alpha.md")
            assert not (w / "logs").exists(), "sync logged before conflict check"
            return {"remote_before": remote_tip, "remote_after": remote_tip, "result": r}
        case("sync/autostash-conflict-preserves-both-sources", autostash)

        for kind in ("missing", "corrupt", "unreadable"):
            def orphan(kind=kind):
                w, _ = fixture("orphan-" + kind)
                info = canopy(w, "checkout", "alpha", mutation=False)
                copy = Path(info["path"])
                copy.write_bytes(copy.read_bytes() + b"unique draft\n")
                meta = copy.with_suffix(".json")
                meta.unlink()
                if kind == "corrupt":
                    meta.write_text("{broken")
                elif kind == "unreadable":
                    meta.mkdir()
                old = digest(copy.read_bytes())
                p = run([binary, "--wiki", w, "--json", "checkout", "alpha"], check=False)
                assert p.returncode != 0
                assert digest(copy.read_bytes()) == old
                info = canopy(w, "checkout", "--list", mutation=False)["checkouts"][0]
                assert info["status"] == "orphan" and info["diagnostic"] and info["rescue"]
                return {"old_copy_hash": old, "after_copy_hash": old, "diagnostic": info}
            case("checkout/orphan/" + kind, orphan)

        def two_wikis():
            a, _ = fixture("isolation-a")
            b, _ = fixture("isolation-b")
            before = {str(p.relative_to(b)): digest(p.read_bytes()) for p in b.rglob("*") if p.is_file()}
            canopy(a, "new", "Isolated", "--type", "concept", "--slug", "isolated")
            after = {str(p.relative_to(b)): digest(p.read_bytes()) for p in b.rglob("*") if p.is_file()}
            assert before == after
            return {"other_wiki_before": before, "other_wiki_after": after}
        case("target/two-wiki-isolation", two_wikis)

        def noop_sync():
            w, remote = fixture("noop-sync", True)
            before = digest((w / "concepts/alpha.md").read_bytes())
            canopy(w, "checkout", "alpha", mutation=False)
            r = canopy(w, "checkin", "alpha", "--sync")
            assert r["status"] == "noop" and r["applied"] == "none" and r["pushed"]
            assert digest((w / "concepts/alpha.md").read_bytes()) == before
            assert git(w, "rev-parse", "HEAD") == git(remote, "rev-parse", "refs/heads/main")
            return {"page_hash": before, "result": r}
        case("checkin/noop-honors-sync", noop_sync)

        for verb in ("checkin", "mv"):
            for sync in (False, True):
                if verb == "checkin" and sync:
                    continue  # the explicit check above already covers this cell
                def other_noop(verb=verb, sync=sync):
                    w, remote = fixture(f"noop-{verb}-{sync}", sync)
                    before = digest((w / "concepts/alpha.md").read_bytes())
                    args = ["alpha", "--slug", "alpha"]
                    if verb == "checkin":
                        canopy(w, "checkout", "alpha", mutation=False)
                        args = ["alpha"]
                    r = canopy(w, verb, *args, *(["--sync"] if sync else []))
                    assert r["status"] == "noop" and r["applied"] == "none"
                    assert digest((w / "concepts/alpha.md").read_bytes()) == before
                    if sync:
                        assert r["pushed"] and git(w, "rev-parse", "HEAD") == git(remote, "rev-parse", "refs/heads/main")
                    return {"page_hash": before, "result": r}
                case(f"{verb}/{'sync' if sync else 'basic'}/noop", other_noop)

        def keyword_fallback():
            w, _ = fixture("keyword-fallback")
            r = canopy(w, "search", "baseline", "--mode", "hybrid", mutation=False)
            assert r["mode"] == "keyword" and r["hits"]
            return {"mode": r["mode"], "hits": len(r["hits"])}
        case("lite/F2-hybrid-keyword-fallback-exit-zero", keyword_fallback)

        def early_parse():
            w, _ = fixture("early-parse")
            before = digest((w / "concepts/alpha.md").read_bytes())
            p = run([binary, "--wiki", w, "--json", "new", "MissingType"], check=False)
            assert p.returncode != 0 and p.stderr and not p.stdout
            after = digest((w / "concepts/alpha.md").read_bytes())
            assert before == after
            return {"old_hash": before, "after_hash": after, "empty_stdout_exception": True}
        case("mutation/early-parser-stderr-nonzero-exception", early_parse)

        def deferred_body_create():
            w, _ = fixture("fifo-create")
            fifo = scratch / "create-body-fifo"
            os.mkfifo(fifo)
            first = subprocess.Popen([str(binary), "--wiki", str(w), "--json", "new",
                                      "Beta", "--type", "concept", "--slug", "beta",
                                      "--body-file", str(fifo)], env=env, text=True,
                                     stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            try:
                second = canopy(w, "new", "Beta", "--type", "concept", "--slug", "beta")
                with fifo.open("w") as out:
                    out.write("late source must not overwrite winner\n")
                stdout, stderr = first.communicate(timeout=30)
                report["commands"].append({"argv": first.args, "exit": first.returncode,
                                           "stdout": stdout, "stderr": stderr})
                r = json.loads(stdout)
                assert first.returncode != 0 and r["applied"] == "none"
                actual = digest((w / "concepts/beta.md").read_bytes())
                assert second["result"]["applied_hash"] == actual
                logs = b"".join(p.read_bytes() for p in (w / "logs").glob("*.jsonl"))
                assert logs.count(b'"action":"create"') == 1
                return {"winner_hash": actual, "loser": r}
            finally:
                if first.poll() is None:
                    first.kill()
                    first.communicate()
        case("create/two-process-input-boundary-no-clobber", deferred_body_create)

        def sync_boundary():
            w, remote = fixture("sync-boundary", True)
            fifo = scratch / "sync-hook-fifo"
            os.mkfifo(fifo)
            hook = w / ".git/hooks/pre-commit"
            hook.write_text(f"#!/bin/sh\nread gate < '{fifo}'\n")
            hook.chmod(0o755)
            syncing = subprocess.Popen([str(binary), "--wiki", str(w), "--json", "sync"],
                                       env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            writer = None
            try:
                # Opening the writer rendezvous proves Git is in its precommit
                # hook while canopy owns the exclusive wiki lease.
                gate = fifo.open("w")
                writer = subprocess.Popen([str(binary), "--wiki", str(w), "--json", "new",
                                           "Late", "--type", "concept", "--slug", "late"],
                                          env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
                time.sleep(0.2)
                assert writer.poll() is None and not (w / "concepts/late.md").exists()
                gate.write("continue\n")
                gate.close()
                sout, serr = syncing.communicate(timeout=30)
                wout, werr = writer.communicate(timeout=30)
                for p, stdout, stderr in ((syncing, sout, serr), (writer, wout, werr)):
                    report["commands"].append({"argv": p.args, "exit": p.returncode,
                                               "stdout": stdout, "stderr": stderr})
                assert syncing.returncode == writer.returncode == 0, (serr, werr)
                s, r = json.loads(sout), json.loads(wout)
                assert s["pushed"] and r["applied"] == "applied"
                assert (w / "concepts/late.md").exists()
                assert "concepts/late.md" not in git(remote, "ls-tree", "-r", "--name-only", "refs/heads/main").splitlines()
                return {"sync": s, "writer": r, "remote_excludes_later_write": True}
            finally:
                for p in (writer, syncing):
                    if p is not None and p.poll() is None:
                        p.kill()
                        p.communicate()
        case("sync/exclusive-git-boundary-defers-cooperating-writer", sync_boundary)

        def stale_scratch():
            w, remote = fixture("stale-scratch", True)
            stale = w / "concepts/.canopy-tmp-12345"
            stale.write_text("abandoned partial scratch")
            r = canopy(w, "sync", mutation=False)
            assert r["committed"] and r["pushed"] and not stale.exists()
            assert not git(w, "status", "--porcelain", "--untracked-files=all")
            assert ".canopy-tmp-" not in git(remote, "ls-tree", "-r", "--name-only", "refs/heads/main")
            return {"result": r, "git_clean": True}
        case("sync/abandoned-scratch-cleaned-before-git", stale_scratch)
    finally:
        if "owned_source_sha256" in report:
            report["owned_sources_stable_during_run"] = all((REPO / name).exists() and digest((REPO / name).read_bytes()) == sha for name, sha in report["owned_source_sha256"].items())
        shutil.rmtree(scratch)
        report["scratch_removed"] = not scratch.exists()
        report["passed"] = sum(c["pass"] for c in report["cases"])
        report["failed"] = sum(not c["pass"] for c in report["cases"])
        OUTPUT.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"passed": report["passed"], "failed": report["failed"], "output": str(OUTPUT)}))
    return 1 if report["failed"] else 0


if __name__ == "__main__":
    sys.exit(main())
