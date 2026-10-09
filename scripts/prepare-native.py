#!/usr/bin/env python3
"""Prepare a pinned tokenizer archive; download/hash/extract are separate steps."""
import argparse
import hashlib
import json
import os
import pathlib
import platform
import shutil
import subprocess
import sys
import tarfile
import tempfile

VERSION = 'v1.27.0'
PINS = {
    'darwin/arm64': ('darwin-arm64', 'fb84b8b2e349a5952767ffe80ccd862fc44084de47f3b0cc3f0b7c9d4e649cf7'),
    'darwin/amd64': ('darwin-x86_64', '6239efe5a81fde8089ef2df8ae710366542b4e5deab6d8ecb74d7d1862db2ddb'),
    'linux/arm64': ('linux-arm64', 'e96545ad05930c26f51f63d932ee6d3bbd32bbed149e102c5290d587a2293067'),
    'linux/amd64': ('linux-amd64', '72556cdca798dd4ea7cdaba308e5f0d68a8cb93b67c96edf485b7a0edd7b07f4'),
}


def host_tuple():
    arch = {'aarch64': 'arm64', 'x86_64': 'amd64'}.get(platform.machine(), platform.machine())
    return f'{platform.system().lower()}/{arch}'


def digest(path):
    h = hashlib.sha256()
    with open(path, 'rb') as f:
        for part in iter(lambda: f.read(1024 * 1024), b''):
            h.update(part)
    return h.hexdigest()


def prepare(libdir, target, curl='curl', pin=None):
    if target not in PINS:
        raise RuntimeError(f'unsupported native host tuple {target}; no download attempted')
    if target.startswith('linux/') and platform.libc_ver()[0] != 'glibc' and pin is None:
        raise RuntimeError('native source build supports Linux glibc only')
    artifact, expected = pin or PINS[target]
    libdir = pathlib.Path(libdir).resolve()
    libdir.mkdir(parents=True, exist_ok=True)
    lock = libdir / '.tokenizers-install-lock'
    try:
        lock.mkdir()
    except FileExistsError:
        raise RuntimeError(f'native installer busy: {lock}')
    try:
        final = libdir / 'libtokenizers.a'
        marker = libdir / 'tokenizers-manifest.json'
        if final.is_file() and final.stat().st_size and marker.is_file():
            try:
                m = json.loads(marker.read_text())
                if (isinstance(m, dict) and m.get('tuple') == target and m.get('version') == VERSION
                        and m.get('archive_sha256') == expected and m.get('library_sha256') == digest(final)):
                    print('native cache verified')
                    return
            except (KeyError, ValueError, OSError):
                pass
        with tempfile.TemporaryDirectory(prefix='.tokenizers-stage-', dir=libdir) as temp:
            stage = pathlib.Path(temp)
            archive = stage / 'archive.tar.gz'
            url = f'https://github.com/daulet/tokenizers/releases/download/{VERSION}/libtokenizers.{artifact}.tar.gz'
            result = subprocess.run([curl, '--fail', '--location', '--silent', '--show-error',
                                     '--connect-timeout', '15', '--max-time', '180', '--retry', '2',
                                     '--output', str(archive), url], check=False, timeout=200)
            if result.returncode:
                raise RuntimeError(f'native download failed: curl exit {result.returncode}')
            if digest(archive) != expected:
                raise RuntimeError('native archive digest mismatch')
            try:
                with tarfile.open(archive, 'r:gz') as tar:
                    candidates = [m for m in tar.getmembers() if pathlib.PurePosixPath(m.name).name == 'libtokenizers.a']
                    if len(candidates) != 1 or not candidates[0].isfile() or not (0 < candidates[0].size <= 256 * 1024 * 1024):
                        raise RuntimeError('native archive lacks one nonempty regular library')
                    source = tar.extractfile(candidates[0])
                    with source, open(stage / 'libtokenizers.a', 'wb') as out:
                        shutil.copyfileobj(source, out)
            except (tarfile.TarError, EOFError, OSError) as e:
                raise RuntimeError(f'native extract failed: {e}') from e
            ready = stage / 'libtokenizers.a'
            m = dict(tuple=target, version=VERSION, archive_sha256=expected, library_sha256=digest(ready))
            staged_marker = stage / 'manifest.json'
            staged_marker.write_text(json.dumps(m, indent=2) + '\n')
            # Single-library publication is atomic. A crash before marker
            # publication causes revalidation on the next invocation.
            os.replace(ready, final)
            os.replace(staged_marker, marker)
            print(f'native ready: {target} {final}')
    finally:
        lock.rmdir()


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--libdir', default=str(pathlib.Path(os.environ.get('XDG_DATA_HOME', pathlib.Path.home() / '.local/share')) / 'canopy/lib'))
    p.add_argument('--host', default=host_tuple())
    args = p.parse_args()
    requested = f'{os.environ.get("GOOS", platform.system().lower())}/{os.environ.get("GOARCH", host_tuple().split("/")[1])}'
    if requested != host_tuple():
        p.error(f'cross-native build {requested} on {host_tuple()} is unsupported; no download attempted')
    if args.host != host_tuple():
        p.error('native assets must match the build host; cross-native builds are unsupported')
    try:
        prepare(args.libdir, args.host)
    except (OSError, RuntimeError, subprocess.TimeoutExpired) as e:
        print(e, file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
