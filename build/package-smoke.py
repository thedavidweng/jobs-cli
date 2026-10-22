#!/usr/bin/env python3
"""Execute a checksummed GoReleaser archive on its native runner."""

import json
import os
from pathlib import Path
import platform
import subprocess
import sys
import tempfile

import release_size as size

directory = Path(sys.argv[1])
goos = {"Darwin": "darwin", "Linux": "linux", "Windows": "windows"}[platform.system()]
arch = {"aarch64": "arm64", "arm64": "arm64", "amd64": "x86_64", "x86_64": "x86_64"}[
    platform.machine().lower()
]
targets = [f"{goos}_{arch}"]
if goos == "darwin":
    targets.append("darwin_universal")

checksums = size.verify_checksums(directory)
metadata = json.loads((directory / "metadata.json").read_text())
with tempfile.TemporaryDirectory() as temporary:
    root = Path(temporary)
    for target in targets:
        extension = "zip" if goos == "windows" else "tar.gz"
        name = f"jobs-cli_{target}.{extension}"
        assert name in checksums, name
        target_root = root / target
        target_root.mkdir()
        binary = target_root / ("jobs-cli.exe" if goos == "windows" else "jobs-cli")
        binary.write_bytes(size.executable(directory / name))
        binary.chmod(0o755)
        env = dict(os.environ, HOME=str(target_root), USERPROFILE=str(target_root),
                   XDG_CONFIG_HOME=str(target_root / "config"))
        for arguments in (["--help"], ["--version"], ["version", "--json"]):
            result = subprocess.run([str(binary), *arguments], env=env, capture_output=True, text=True, check=True)
            assert result.stdout.strip(), (target, arguments)
            if arguments == ["version", "--json"]:
                data = json.loads(result.stdout)["data"]
                assert data["built_by"] == "goreleaser", data
                assert data["commit"] not in ("none", ""), data
                assert metadata["commit"].startswith(data["commit"]), data
                assert data["date"] not in ("unknown", ""), data
                assert data["version"] not in ("dev", ""), data
                assert data["version"] == metadata["version"], data
                assert data["go_version"].startswith("go1."), data
                assert "github.com/thedavidweng/jobs-cli/v2" in data["build_info"], data
            print(target, arguments, result.stdout.strip()[:500])
        failure = subprocess.run([str(binary), "not-a-command"], env=env, capture_output=True, text=True)
        assert failure.returncode != 0, failure
        assert 'unknown command "not-a-command"' in failure.stderr, failure
        print(target, "unknown command correctly returned", failure.returncode)
