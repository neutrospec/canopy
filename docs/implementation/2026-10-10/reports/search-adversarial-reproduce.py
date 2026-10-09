#!/usr/bin/env python3
"""Run independent review probes in temporary HOME/XDG; never edits implementation.

Go overlay adds inert review tests to package discovery. No real model download,
wiki, or installed native/model asset is used. Exit 1 means a regression probe
failed; each probe includes a passing healthy control.
"""
import argparse
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
from unittest import mock

sys.dont_write_bytecode = True
REPORTS = Path(__file__).resolve().parent
ROOT = REPORTS.parents[3]


def load_script(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def native_marker_probe(temp):
    native = load_script('review_native', ROOT / 'scripts/prepare-native.py')
    archive = temp / 'toy.tar.gz'
    payload = b'new verified library fixture'
    with tarfile.open(archive, 'w:gz') as tar:
        entry = tarfile.TarInfo('libtokenizers.a')
        entry.size = len(payload)
        tar.addfile(entry, io.BytesIO(payload))
    pin = ('review-toy', hashlib.sha256(archive.read_bytes()).hexdigest())
    records = []
    for label, marker in [('object-control', '{}'), ('null-marker', 'null'), ('array-marker', '[]')]:
        lib = temp / label
        lib.mkdir()
        (lib / 'libtokenizers.a').write_bytes(b'old normal library fixture')
        (lib / 'tokenizers-manifest.json').write_text(marker)
        calls = []
        def download(argv, **kwargs):
            calls.append(argv)
            shutil.copyfile(archive, argv[argv.index('--output') + 1])
            return subprocess.CompletedProcess(argv, 0)
        error = None
        with mock.patch.object(native.subprocess, 'run', side_effect=download):
            try:
                native.prepare(lib, native.host_tuple(), pin=pin)
            except Exception as exc:
                error = f'{type(exc).__name__}: {exc}'
        record = dict(probe='native-marker-retry', label=label, error=error,
                      download_calls=len(calls), final_bytes=(lib / 'libtokenizers.a').read_text())
        records.append(record)
        print(json.dumps(record))
    return records


def source_hashes():
    names = ['internal/embed/model.go', 'internal/embed/profile.go', 'internal/embed/onnx.go',
             'internal/embed/validation.go', 'internal/embed/chunk.go', 'internal/embed/engine.go',
             'internal/embed/engine_ort.go', 'internal/indexer/indexer.go', 'internal/store/store.go',
             'internal/store/vectors.go', 'internal/search/search.go', 'internal/search/evaluation_test.go',
             'internal/search/testdata/evaluation/corpus.json', 'scripts/prepare-native.py',
             'scripts/quantize-model.py', 'cmd/canopy/main.go', 'cmd/canopy/recall.go',
             'cmd/canopy/write.go', 'cmd/canopy/resurface.go', 'cmd/canopy/reconcile.go',
             'internal/webui/server.go', 'internal/resurface/pick.go', 'scripts/test-go.sh']
    return {n: hashlib.sha256((ROOT / n).read_bytes()).hexdigest() for n in names}


def quantizer_probe(temp, env):
    source = temp / 'quantizer-source'
    source.mkdir()
    (source / 'model.onnx').write_bytes(bytes([58, 0]))
    for name in ['config.json', 'tokenizer.json']:
        (source / name).write_text('{}')
    records = []
    for label, graph, key in [
        ('inline-control', bytes([58,0]), 'CANOPY_REVIEW_CONVERTED_GOOD'),
        ('missing-sidecar', bytes.fromhex('3a1d2a1b6a170a086c6f636174696f6e120b776569676874732e62696e7001'), 'CANOPY_REVIEW_CONVERTED_BAD'),
    ]:
        destination = temp / ('converted-' + label)
        shutil.copytree(source, destination)
        converter = temp / ('converter-' + label)
        converter.write_text('#!' + sys.executable + '\nimport sys\nfrom pathlib import Path\n'
                             'Path(sys.argv[2]).write_bytes(bytes.fromhex(' + repr(graph.hex()) + '))\n'
                             + ('Path(sys.argv[2]).with_name("partial.bin").write_bytes(b"partial")\n' if label=='missing-sidecar' else ''))
        converter.chmod(0o755)
        argv = [sys.executable, str(ROOT / 'scripts/quantize-model.py'), '--source', str(source),
                '--destination', str(destination), '--converter', str(converter)]
        result = subprocess.run(argv, env=env, capture_output=True, text=True, timeout=30)
        env[key] = str(destination)
        rec = dict(probe='quantizer-publication', label=label, argv=argv, exit=result.returncode,
                   stdout=result.stdout, stderr=result.stderr,
                   active_files=sorted(p.name for p in destination.iterdir()),
                   backup_dirs=[str(p) for p in temp.glob('.' + destination.name + '.previous-*')])
        records.append(rec)
        print(json.dumps(rec), flush=True)
    return records


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--baseline', action='store_true')
    p.add_argument('--output', type=Path, default=REPORTS / 'search-adversarial-results.json')
    args = p.parse_args()
    record = {'source_before': source_hashes(), 'commands': [], 'python_probes': []}
    with tempfile.TemporaryDirectory(prefix='canopy-search-adversarial-', dir='/private/tmp') as raw:
        temp = Path(raw)
        env = dict(os.environ)
        for key, sub in [('HOME', 'home'), ('XDG_CONFIG_HOME', 'config'), ('XDG_CACHE_HOME', 'cache'),
                         ('XDG_STATE_HOME', 'state'), ('XDG_DATA_HOME', 'data'), ('TMPDIR', 'tmp')]:
            path = temp / sub
            path.mkdir()
            env[key] = str(path)
        env.update(GOMODCACHE='/Users/nobocop/go/pkg/mod',
                   GOCACHE='/private/tmp/canopy-improve-20261010/go-build',
                   GOTELEMETRY='off', GOPROXY='off', GOTOOLCHAIN='local', GOFLAGS='-mod=readonly',
                   PYTHONDONTWRITEBYTECODE='1')
        env.pop('CANOPY_TEST_MODEL', None)
        env.pop('CANOPY_TEST_ENV_ACTIVE', None)
        record['python_probes'] += quantizer_probe(temp, env)
        record['environment'] = {k: env[k] for k in ['HOME','XDG_CONFIG_HOME','XDG_CACHE_HOME',
            'XDG_STATE_HOME','XDG_DATA_HOME','TMPDIR','GOMODCACHE','GOCACHE','GOTELEMETRY','GOPROXY',
            'GOTOOLCHAIN','GOFLAGS']}
        overlay = {'Replace': {}}
        for package in ['embed', 'indexer', 'store', 'search', 'resurface', 'cli']:
            fixture = REPORTS / f'search-adversarial-{package}.go.txt'
            if fixture.exists():
                target = ROOT / ('cmd/canopy' if package == 'cli' else f'internal/{package}') / 'search_adversarial_review_test.go'
                overlay['Replace'][str(target)] = str(fixture)
        overlay_path = temp / 'overlay.json'
        overlay_path.write_text(json.dumps(overlay))
        packages = ['./internal/embed', './internal/indexer', './internal/store', './internal/search', './internal/resurface', './cmd/canopy']
        commands = [
            ['bash','scripts/test-go.sh','-overlay',str(overlay_path),'-list','^TestAdversarial',*packages],
            ['bash','scripts/test-go.sh','-overlay',str(overlay_path),'-count=1','-run','^TestAdversarial','-v',*packages],
        ]
        if args.baseline:
            commands += [['bash','scripts/test-go.sh','-count=1',*packages],
                         [sys.executable,'scripts/test-native-download.py']]
        for argv in commands:
            print('COMMAND ' + json.dumps(argv), flush=True)
            result = subprocess.run(argv, cwd=ROOT, env=env, text=True, capture_output=True, timeout=300)
            rec = dict(argv=argv, exit=result.returncode, stdout=result.stdout, stderr=result.stderr)
            record['commands'].append(rec)
            print(result.stdout, end='')
            print(result.stderr, end='', file=sys.stderr)
            print(f'EXIT {result.returncode}', flush=True)
            args.output.write_text(json.dumps(record, indent=2) + '\n')
        record['python_probes'] += native_marker_probe(temp)
    record['source_after'] = source_hashes()
    args.output.write_text(json.dumps(record, indent=2) + '\n')
    return int(any(r['exit'] for r in record['commands']) or any(r.get('error') for r in record['python_probes']))


if __name__ == '__main__':
    sys.exit(main())
