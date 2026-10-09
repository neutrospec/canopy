#!/usr/bin/env python3
"""Bounded native download/conversion failure tests; no real assets downloaded."""
import hashlib
import importlib.util
import io
import json
import os
import pathlib
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location('native', ROOT / 'scripts/prepare-native.py')
native = importlib.util.module_from_spec(spec)
spec.loader.exec_module(native)


class NativeTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='canopy-native-tests-')
        self.addCleanup(self.temp.cleanup)
        self.root = pathlib.Path(self.temp.name)
        self.libdir = self.root / 'lib'
        self.libdir.mkdir()
        self.old = self.libdir / 'libtokenizers.a'
        self.old.write_bytes(b'previous-good-library')
        self.archive = self.root / 'toy.tar.gz'
        with tarfile.open(self.archive, 'w:gz') as tar:
            b = b'toy-new-library'
            m = tarfile.TarInfo('libtokenizers.a')
            m.size = len(b)
            tar.addfile(m, io.BytesIO(b))
        self.digest = hashlib.sha256(self.archive.read_bytes()).hexdigest()

    def fake_curl(self, status=0, payload=None):
        def run(argv, **kwargs):
            dst = pathlib.Path(argv[argv.index('--output') + 1])
            dst.write_bytes(self.archive.read_bytes() if payload is None else payload)
            return subprocess.CompletedProcess(argv, status)
        return run

    def test_complete_tar_transfer_failure_preserves_old(self):
        with mock.patch.object(native.subprocess, 'run', side_effect=self.fake_curl(18)):
            with self.assertRaisesRegex(RuntimeError, 'download failed'):
                native.prepare(self.libdir, 'darwin/arm64', pin=('toy', self.digest))
        self.assertEqual(self.old.read_bytes(), b'previous-good-library')
        self.assertFalse((self.libdir / 'tokenizers-manifest.json').exists())

    def test_hash_and_extract_failures_preserve_old(self):
        for payload, digest, reason in ((None, '0' * 64, 'digest mismatch'),
                                         (b'invalid archive', hashlib.sha256(b'invalid archive').hexdigest(), 'extract failed')):
            with self.subTest(reason=reason), mock.patch.object(native.subprocess, 'run', side_effect=self.fake_curl(payload=payload)):
                with self.assertRaisesRegex(RuntimeError, reason):
                    native.prepare(self.libdir, 'darwin/arm64', pin=('toy', digest))
                self.assertEqual(self.old.read_bytes(), b'previous-good-library')

    def test_retry_success_cache_and_concurrency(self):
        with mock.patch.object(native.subprocess, 'run', side_effect=self.fake_curl()) as transfer:
            native.prepare(self.libdir, 'darwin/arm64', pin=('toy', self.digest))
            self.assertEqual(self.old.read_bytes(), b'toy-new-library')
            native.prepare(self.libdir, 'darwin/arm64', pin=('toy', self.digest))
            self.assertEqual(transfer.call_count, 1)
            (self.libdir / '.tokenizers-install-lock').mkdir()
            with self.assertRaisesRegex(RuntimeError, 'busy'):
                native.prepare(self.libdir, 'darwin/arm64', pin=('toy', self.digest))

    def test_unsupported_host_does_no_work(self):
        with mock.patch.object(native.subprocess, 'run') as transfer:
            with self.assertRaisesRegex(RuntimeError, 'unsupported'):
                native.prepare(self.libdir, 'windows/arm64')
            transfer.assert_not_called()
        self.assertEqual(self.old.read_bytes(), b'previous-good-library')

    def test_nonobject_marker_retries_with_old_library_preserved_on_failure(self):
        for raw in ('null', '[]', '{}'):
            with self.subTest(marker=raw):
                (self.libdir / 'tokenizers-manifest.json').write_text(raw)
                with mock.patch.object(native.subprocess, 'run', side_effect=self.fake_curl(18)) as transfer:
                    with self.assertRaisesRegex(RuntimeError, 'download failed'):
                        native.prepare(self.libdir, 'darwin/arm64', pin=('toy', self.digest))
                    self.assertEqual(transfer.call_count, 1)
                self.assertEqual(self.old.read_bytes(), b'previous-good-library')

    def test_quantizer_xdg_explicit_and_failed_converter(self):
        env = dict(os.environ, HOME=str(self.root / 'home'), XDG_DATA_HOME=str(self.root / 'xdg'))
        script = ROOT / 'scripts/quantize-model.py'
        result = subprocess.run([sys.executable, str(script), '--print-paths'], env=env, capture_output=True, text=True, check=True)
        paths = json.loads(result.stdout)
        self.assertEqual(paths['source'], str(self.root / 'xdg/canopy/models/bge-m3'))
        source, destination = self.root / 'source', self.root / 'destination'
        source.mkdir()
        (source / 'model.onnx').write_bytes(b'\x3a\x00')
        for name in ('config.json', 'tokenizer.json'):
            (source / name).write_text('{}')
        destination.mkdir()
        (destination / 'old').write_text('keep')
        converter = self.root / 'fake-converter'
        converter.write_text('#!' + sys.executable + '\nimport sys\nfrom pathlib import Path\nPath(sys.argv[2]).write_bytes(bytes([58,0]))\n')
        converter.chmod(0o755)
        argv = [sys.executable, str(script), '--source', str(source), '--destination', str(destination), '--converter', str(converter)]
        subprocess.run(argv, env=env, check=True, capture_output=True)
        self.assertTrue((destination / 'canopy-model.json').exists())
        before = (destination / 'canopy-model.json').read_bytes()
        converter.write_text('#!' + sys.executable + '\nimport sys\nsys.exit(9)\n')
        result = subprocess.run(argv, env=env, capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((destination / 'canopy-model.json').read_bytes(), before)
        self.assertFalse(list(self.root.glob('.destination.stage-*')))
        old_files = {p.relative_to(destination).as_posix(): p.read_bytes() for p in destination.rglob('*') if p.is_file()}
        # Converter exits successfully but declares a missing sidecar. It must
        # fail before moving the current ready bundle into a backup directory.
        graph = '3a1d2a1b6a170a086c6f636174696f6e120b776569676874732e62696e7001'
        converter.write_text('#!' + sys.executable + '\nimport sys\nfrom pathlib import Path\nPath(sys.argv[2]).write_bytes(bytes.fromhex(' + repr(graph) + '))\n')
        backups = set(self.root.glob('.destination.previous-*'))
        result = subprocess.run(argv, env=env, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('external artifact', result.stderr)
        self.assertEqual(old_files, {p.relative_to(destination).as_posix(): p.read_bytes() for p in destination.rglob('*') if p.is_file()})
        self.assertEqual(backups, set(self.root.glob('.destination.previous-*')))


if __name__ == '__main__':
    unittest.main()
