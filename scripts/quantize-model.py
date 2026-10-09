#!/usr/bin/env python3
"""Offline bge-m3 conversion using runtime-compatible XDG or explicit paths."""
import argparse
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile

from onnx_bundle import external_locations


def default_paths():
    data = pathlib.Path(os.environ.get('XDG_DATA_HOME', pathlib.Path.home() / '.local/share'))
    models = data / 'canopy/models'
    return models / 'bge-m3', models / 'bge-m3-int8'


def sha(path):
    h = hashlib.sha256()
    with open(path, 'rb') as f:
        for block in iter(lambda: f.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def convert(source, destination, converter=None):
    source, destination = pathlib.Path(source).resolve(), pathlib.Path(destination).absolute()
    if source == destination.resolve():
        raise RuntimeError('source and destination must differ')
    if destination.is_symlink():
        raise RuntimeError('destination is a symlink; specify its resolved path')
    for name in ('model.onnx', 'config.json', 'tokenizer.json'):
        path = source / name
        if not path.is_file() or path.stat().st_size == 0:
            raise RuntimeError(f'missing/nonempty source file required: {name}')
        if name.endswith('.json') and not isinstance(json.loads(path.read_text()), dict):
            raise RuntimeError(f'{name} must be a JSON object')
    destination.parent.mkdir(parents=True, exist_ok=True)
    lock = destination.parent.resolve() / ('.' + destination.name + '.install-lock')
    try:
        lock.mkdir()
    except FileExistsError:
        raise RuntimeError(f'model installer busy: {lock}')
    try:
        with tempfile.TemporaryDirectory(prefix='.' + destination.name + '.stage-', dir=destination.parent) as tmp:
            stage = pathlib.Path(tmp)
            if converter:
                subprocess.run([str(converter), str(source / 'model.onnx'), str(stage / 'model.onnx')], check=True, timeout=1800)
            else:
                import onnx
                from onnxruntime.quantization import quantize_dynamic, QuantType
                quantize_dynamic(model_input=str(source / 'model.onnx'), model_output=str(stage / 'model.onnx'),
                                 weight_type=QuantType.QInt8, use_external_data_format=True,
                                 extra_options={'MatMulConstBOnly': True, 'DefaultTensorType': onnx.TensorProto.FLOAT})
            refs = external_locations(stage / 'model.onnx')
            kind = 'onnx-external' if refs else 'onnx-inline'
            for name in refs:
                artifact = stage / name
                if artifact.is_symlink() or not artifact.is_file() or artifact.stat().st_size == 0:
                    raise RuntimeError(f'converter omitted nonempty external artifact: {name}')
            for name in ('config.json', 'tokenizer.json', 'tokenizer_config.json', 'special_tokens_map.json'):
                if (source / name).exists():
                    shutil.copy2(source / name, stage / name)
            files = {}
            for path in stage.rglob('*'):
                if path.is_file():
                    if not path.stat().st_size:
                        raise RuntimeError(f'converter produced empty artifact: {path.name}')
                    if path.suffix == '.json' and not isinstance(json.loads(path.read_text()), dict):
                        raise RuntimeError(f'converter produced invalid JSON: {path.name}')
                    files[path.relative_to(stage).as_posix()] = {'sha256': sha(path), 'size': path.stat().st_size}
            if 'model.onnx' not in files:
                raise RuntimeError('converter did not produce model.onnx')
            revision = hashlib.sha256(json.dumps(files, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
            manifest = dict(version=1, repository='local-int8', revision='local-' + revision, kind=kind,
                            pooling='first-output/mean-if-3d/l2', dimension=1024, files=files)
            (stage / 'canopy-model.json').write_text(json.dumps(manifest, indent=2) + '\n')
            backup = None
            if destination.exists():
                backup = pathlib.Path(tempfile.mkdtemp(prefix='.' + destination.name + '.previous-', dir=destination.parent))
                backup.rmdir()
                os.rename(destination, backup)
            try:
                os.rename(stage, destination)
            except OSError:
                if backup:
                    os.rename(backup, destination)
                raise
            if backup:
                print(f'previous bundle retained: {backup}')
            print(f'done: {destination} ({sum(f["size"] for f in files.values())} bytes)')
    finally:
        lock.rmdir()


def main():
    src, dst = default_paths()
    p = argparse.ArgumentParser(description=__doc__, epilog='Default conversion requires Python packages onnx and onnxruntime. --converter selects an explicit offline converter instead. Select the output with [embedding].model in canopy.toml, then run model status and reindex for that wiki.')
    p.add_argument('--source', '--src', default=str(src))
    p.add_argument('--destination', '--dst', default=str(dst))
    p.add_argument('--converter', type=pathlib.Path, help='explicit converter executable for controlled offline/test use')
    p.add_argument('--print-paths', action='store_true', help='resolve paths without importing ONNX or converting')
    args = p.parse_args()
    if args.print_paths:
        print(json.dumps({'source': str(pathlib.Path(args.source).absolute()), 'destination': str(pathlib.Path(args.destination).absolute())}))
        return 0
    try:
        convert(args.source, args.destination, args.converter)
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError, ImportError) as e:
        print(e, file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
