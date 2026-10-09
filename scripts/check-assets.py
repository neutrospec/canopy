#!/usr/bin/env python3
"""Check pinned web assets and notices; optionally inspect a release archive.

No network or archive extraction. This is a bounded provenance/packaging check,
not an SBOM, a vulnerability scan, or a browser-rendering test.
"""
import argparse
import fnmatch
import hashlib
import io
import json
from pathlib import Path, PurePosixPath
import tarfile
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parent.parent
MANIFEST = "internal/webui/static/vendor/assets.json"


def digest(data):
    return hashlib.sha256(data).hexdigest()


def source_errors(root, manifest):
    errors = []
    for item in manifest["assets"] + manifest["required_notices"]:
        path = root / item["path"]
        if not path.is_file():
            errors.append(f"missing {item['path']}")
        elif digest(path.read_bytes()) != item["sha256"]:
            errors.append(f"hash mismatch {item['path']}")
    return errors


def packaging_errors(root, manifest):
    # Declarations are checked separately from an actual archive's contents.
    release = (root / ".goreleaser.yaml").read_text()
    formula = (root / "packaging/homebrew/canopy.rb").read_text()
    patterns = [line.strip()[2:] for line in release.splitlines() if line.strip().startswith("- ")]
    errors = []
    for item in manifest["required_notices"] + [{"path": MANIFEST}]:
        path = item["path"]
        if not any(fnmatch.fnmatchcase(path, pattern) for pattern in patterns):
            errors.append(f"GoReleaser declaration missing {path}")
    for declaration in ('"THIRD_PARTY_NOTICES.md"', '"internal/webui/static/vendor/{LICENSE,NOTICE}-*.txt"', '"internal/webui/static/vendor/assets.json"', '(pkgshare/"internal/webui/static/vendor")', '"internal/webui/static/vendor/fonts/LICENSE-fonts.txt"'):
        if declaration not in formula:
            errors.append(f"Homebrew declaration missing {declaration}")
    return errors


def archive_members(path):
    members = {}
    if zipfile.is_zipfile(path):
        with zipfile.ZipFile(path) as archive:
            files = [(entry.filename, archive.read(entry)) for entry in archive.infolist() if not entry.is_dir()]
    else:
        with tarfile.open(path, "r:*") as archive:
            files = [(entry.name, archive.extractfile(entry).read()) for entry in archive.getmembers() if entry.isfile()]
    for name, data in files:
        normalized = str(PurePosixPath(name))
        if normalized in members:
            raise ValueError(f"duplicate archive member {normalized}")
        members[normalized] = data
    return members


def archive_errors(path, manifest):
    members = archive_members(path)
    errors = []
    expected_files = manifest["required_notices"] + [{"path": MANIFEST, "sha256": digest((ROOT / MANIFEST).read_bytes())}]
    for item in expected_files:
        # GoReleaser may prefix the archive's root directory; directories below
        # it are retained. A same-named file elsewhere is not a substitute.
        expected = item["path"]
        matches = [data for name, data in members.items() if name == expected or name.endswith("/" + expected)]
        if len(matches) != 1:
            errors.append(f"archive requires one {expected}; found {len(matches)}")
        elif digest(matches[0]) != item["sha256"]:
            errors.append(f"archive hash mismatch {expected}")
    return errors


def self_test(root, manifest):
    # Negative controls verify behavior, not string presence in this script.
    with tempfile.TemporaryDirectory(prefix="canopy-assets-") as temp:
        fixture = Path(temp)
        for item in manifest["assets"] + manifest["required_notices"]:
            path = fixture / item["path"]
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes((root / item["path"]).read_bytes())
        assert not source_errors(fixture, manifest)
        asset = fixture / manifest["assets"][0]["path"]
        asset.write_bytes(asset.read_bytes() + b"\nchanged")
        assert any("hash mismatch" in error for error in source_errors(fixture, manifest))
        notice = fixture / manifest["required_notices"][1]["path"]
        notice.unlink()
        assert any("missing" in error for error in source_errors(fixture, manifest))

        def write_archive(name, skip=None, changed=None):
            path = fixture / name
            with tarfile.open(path, "w:gz") as archive:
                for item in manifest["required_notices"] + [{"path": MANIFEST}]:
                    if item["path"] == skip:
                        continue
                    data = (root / item["path"]).read_bytes()
                    if item["path"] == changed:
                        data += b"changed"
                    entry = tarfile.TarInfo("release/" + item["path"])
                    entry.size = len(data)
                    archive.addfile(entry, io.BytesIO(data))
            return path

        assert not archive_errors(write_archive("good.tar.gz"), manifest)
        assert archive_errors(write_archive("missing.tar.gz", skip=manifest["required_notices"][1]["path"]), manifest)
        assert archive_errors(write_archive("changed.tar.gz", changed=manifest["required_notices"][1]["path"]), manifest)
        assert archive_errors(write_archive("missing-manifest.tar.gz", skip=MANIFEST), manifest)
        assert archive_errors(write_archive("changed-manifest.tar.gz", changed=MANIFEST), manifest)
    return 8


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--archive", type=Path, help="inspect notices inside this tar/zip release archive")
    args = parser.parse_args()
    manifest = json.loads((ROOT / MANIFEST).read_text())
    errors = source_errors(ROOT, manifest) + packaging_errors(ROOT, manifest)
    if args.archive:
        try:
            errors += archive_errors(args.archive, manifest)
        except (OSError, ValueError, tarfile.TarError, zipfile.BadZipFile) as error:
            errors.append(f"archive: {error}")
    if errors:
        for error in errors:
            print("FAIL:", error)
        return 1
    checks = self_test(ROOT, manifest)
    print(f"PASS: {len(manifest['assets'])} pinned assets, {len(manifest['required_notices'])} notices, packaging declarations, {checks} positive/negative controls")
    if args.archive:
        print(f"PASS: actual archive notices and manifest: {args.archive}")
    else:
        print("UNVERIFIED: actual release archive (supply --archive PATH)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
