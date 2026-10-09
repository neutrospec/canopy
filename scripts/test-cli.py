#!/usr/bin/env python3
"""Execute the lite CLI's public contracts in two disposable, spaced wiki paths."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', type=Path)
    args = parser.parse_args()
    gopath = os.environ.get('GOPATH', str(Path.home() / 'go')).split(os.pathsep)[0]
    modcache = os.environ.get('GOMODCACHE', str(Path(gopath) / 'pkg/mod'))
    with tempfile.TemporaryDirectory(prefix='canopy CLI ') as temporary:
        base = Path(temporary)
        env = dict(os.environ, CGO_ENABLED='0', GOTELEMETRY='off', GOMODCACHE=modcache)
        env.pop('CANOPY_WIKI', None)
        for key, name in [('HOME', 'home'), ('XDG_CONFIG_HOME', 'config'),
                          ('XDG_CACHE_HOME', 'cache'), ('XDG_STATE_HOME', 'state'),
                          ('XDG_DATA_HOME', 'data'), ('TMPDIR', 'tmp')]:
            path = base / name
            path.mkdir()
            env[key] = str(path)
        env.setdefault('GOCACHE', str(base / 'go-build'))
        binary = args.binary.resolve() if args.binary else base / 'canopy'
        if not args.binary:
            subprocess.run(['go', 'build', '-buildvcs=false', '-o', str(binary), './cmd/canopy'],
                           cwd=ROOT, env=env, check=True)
        wiki, other = base / 'wiki one', base / 'wiki two'
        checks = []

        def run(argv, expected=0, target=wiki, parse=True):
            completed = subprocess.run([str(binary), '--wiki', str(target), '--json', *argv],
                                       cwd=base, env=env, text=True, capture_output=True, timeout=60)
            assert (completed.returncode == 0) == (expected == 0), (argv, completed.returncode, completed.stdout, completed.stderr)
            result = json.loads(completed.stdout) if parse else None
            if parse:
                assert isinstance(result, dict), (argv, result)
            checks.append(' '.join(argv))
            return result, completed

        def git(*argv, directory=wiki):
            return subprocess.check_output(['git', '-C', str(directory), *argv], env=env, text=True, stderr=subprocess.STDOUT).strip()

        def snapshot(directory):
            return {str(p.relative_to(directory)): hashlib.sha256(p.read_bytes()).hexdigest()
                    for p in directory.rglob('*') if p.is_file() and '.git' not in p.parts}

        for directory in (wiki, other):
            directory.mkdir()
            run(['init'], target=directory)
        other_before = snapshot(other)
        remote = base / 'remote.git'
        subprocess.run(['git', 'init', '--bare', str(remote)], env=env, check=True, capture_output=True)
        git('init', '-b', 'main')
        git('config', 'user.name', 'Canopy fixture')
        git('config', 'user.email', 'fixture@example.invalid')
        git('add', '-A')
        git('commit', '-m', 'fixture')
        git('remote', 'add', 'origin', str(remote))
        git('push', '-u', 'origin', 'main')
        body = base / 'input.md'
        body.write_text('keyword sentinel body\n', encoding='utf-8')
        for sync in (False, True):
            suffix = 'sync' if sync else 'basic'
            a, b, c = 'a-' + suffix, 'b-' + suffix, 'c-' + suffix
            options = ['--sync'] if sync else []
            new = ['new', 'Fixture ' + suffix, '--slug', a, '--type', 'concept', '--tags', 'tool', '--body-file', str(body)]
            result, _ = run(new + options)
            assert result['applied'] == 'applied' and result['command'] == 'new'
            old = snapshot(wiki)
            result, _ = run(new + options, expected=1)
            assert result['applied'] == 'none' and snapshot(wiki) == old
            run(['update', a, '--body-file', str(body), *options])
            checkout, _ = run(['checkout', a])
            copy = Path(checkout['path'])
            backup = base / ('backup-' + suffix + '.md')
            backup.write_bytes(copy.read_bytes())
            copy.write_text(copy.read_text() + '\nEdited by fixture.\n')
            result, _ = run(['checkin', a, *options])
            assert result['applied'] == 'applied' and not copy.exists() and backup.exists()
            run(['mv', a, '--slug', b, *options])
            run(['archive', b, *options])
            run(['new', 'Delete fixture', '--slug', c, '--type', 'concept', *options])
            run(['rm', c, *options])
            for verb in ('update', 'mv', 'archive', 'rm', 'checkin'):
                before = snapshot(wiki)
                result, _ = run([verb, 'absent', *options], expected=1)
                assert result['applied'] == 'none' and snapshot(wiki) == before
        run(['new', 'Search fixture', '--slug', 'search-fixture', '--type', 'concept', '--body-file', str(body)])
        result, completed = run(['search', 'sentinel', '--mode', 'hybrid', '--no-mark'])
        assert result['mode'] == 'keyword' and result['hits'] and 'hybrid' in completed.stderr
        before = snapshot(wiki)
        run(['search', 'sentinel', '-k', '-1'], expected=1, parse=False)
        assert snapshot(wiki) == before
        # Source publication succeeds before a deliberately broken log path.
        logs = wiki / 'logs'
        logs.rename(wiki / 'saved-logs')
        logs.write_text('blocked')
        result, _ = run(['new', 'Partial fixture', '--slug', 'partial-fixture', '--type', 'concept', '--body-file', str(body)], expected=1)
        assert result['applied'] == 'applied' and result['stage'] == 'log'
        assert (wiki / 'concepts/partial-fixture.md').exists() and body.read_text() == 'keyword sentinel body\n'
        logs.unlink()
        (wiki / 'saved-logs').rename(logs)
        # Commit and push failures report actual page/local/remote effects.
        remote_before = git('rev-parse', 'refs/heads/main', directory=remote)
        hook = wiki / '.git/hooks/pre-commit'
        hook.write_text('#!/bin/sh\nexit 1\n')
        hook.chmod(0o755)
        result, _ = run(['new', 'Commit fail', '--slug', 'commit-fail', '--type', 'concept', '--sync'], expected=1)
        assert result['applied'] == 'applied' and result['stage'] == 'commit' and not result['committed']
        assert git('rev-parse', 'refs/heads/main', directory=remote) == remote_before
        hook.unlink()
        run(['sync'])
        remote_before = git('rev-parse', 'refs/heads/main', directory=remote)
        hook = wiki / '.git/hooks/pre-push'
        hook.write_text('#!/bin/sh\nexit 1\n')
        hook.chmod(0o755)
        result, _ = run(['new', 'Push fail', '--slug', 'push-fail', '--type', 'concept', '--sync'], expected=1)
        assert result['applied'] == 'applied' and result['stage'] == 'push' and result['committed'] and not result['pushed']
        assert git('rev-parse', 'refs/heads/main', directory=remote) == remote_before
        assert snapshot(other) == other_before
        print(json.dumps({'checks': len(checks), 'lite_hybrid_fallback': 'pass',
                          'single_object_mutations': 'pass', 'failure_effects': 'pass',
                          'non_target_wiki': 'unchanged'}, ensure_ascii=False))


if __name__ == '__main__':
    main()
