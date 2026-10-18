#!/usr/bin/env python3
"""Compare actual GoReleaser archives built from the same source/toolchain."""

import hashlib
import json
from pathlib import Path
import sys
import tarfile
import zipfile

TARGETS = {
    "darwin_x86_64": "darwin-native_darwin_amd64_v1/jobs-cli",
    "darwin_arm64": "darwin-native_darwin_arm64_v8.0/jobs-cli",
    "darwin_universal": "jobs-cli-universal_darwin_all/jobs-cli",
    "linux_x86_64": "jobs-cli_linux_amd64_v1/jobs-cli",
    "linux_arm64": "jobs-cli_linux_arm64_v8.0/jobs-cli",
    "windows_x86_64": "jobs-cli_windows_amd64_v1/jobs-cli.exe",
    "windows_arm64": "jobs-cli_windows_arm64_v8.0/jobs-cli.exe",
}


def verify_checksums(directory):
    checksums = {}
    for line in (directory / "checksums.txt").read_text().splitlines():
        digest, name = line.split()
        path = directory / name
        assert hashlib.sha256(path.read_bytes()).hexdigest() == digest, name
        checksums[name] = digest
    return checksums


def executable(archive):
    name = "jobs-cli.exe" if archive.suffix == ".zip" else "jobs-cli"
    if archive.suffix == ".zip":
        with zipfile.ZipFile(archive) as package:
            members = [member for member in package.namelist() if Path(member).name == name]
            assert len(members) == 1, (archive, members)
            return package.read(members[0])
    with tarfile.open(archive) as package:
        members = [member for member in package.getmembers() if Path(member.name).name == name]
        assert len(members) == 1 and members[0].isfile(), (archive, members)
        return package.extractfile(members[0]).read()


def delta(before, after):
    return f"{(after - before) / 1048576:+.3f} MiB ({(after / before - 1) * 100:+.2f}%)"


def report(baseline, current):
    old_metadata = json.loads((baseline / "metadata.json").read_text())
    new_metadata = json.loads((current / "metadata.json").read_text())
    for field in ("commit", "version", "tag"):
        assert old_metadata[field] == new_metadata[field], field
    old_checksums = verify_checksums(baseline)
    new_checksums = verify_checksums(current)
    expected = {
        f"jobs-cli_{target}.{'zip' if target.startswith('windows') else 'tar.gz'}"
        for target in TARGETS
    }
    assert set(new_checksums) == expected, (set(new_checksums), expected)
    assert set(old_checksums) == expected - {
        "jobs-cli_darwin_x86_64.tar.gz", "jobs-cli_darwin_arm64.tar.gz",
    }, old_checksums
    for target in ("amd64_v1", "arm64_v8.0"):
        native = current / f"darwin-native_darwin_{target}/jobs-cli"
        universal_input = current / f"universal-slices_darwin_{target}/jobs-cli"
        assert native.read_bytes() == universal_input.read_bytes(), target
    rows = [
        "# Release size comparison",
        "",
        "Same source, Go/GoReleaser versions and linker metadata. Baseline disables",
        "`-trimpath` and replaces Darwin slices with the universal archive.",
        "Current enables `-trimpath` and retains both slices and universal.",
        "Archive deltas compare identical targets only; new Darwin archives have no baseline.",
        "All archives passed SHA-256 and archived-executable identity checks.",
        "Snapshots do not prove Apple notarization or published Homebrew acceptance.",
        "",
        "| Target | Raw baseline bytes | Raw current bytes | Raw delta | Archive baseline bytes | Archive current bytes | Archive delta |",
        "| --- | ---: | ---: | --- | ---: | ---: | --- |",
    ]
    for target, binary_path in TARGETS.items():
        name = f"jobs-cli_{target}.{'zip' if target.startswith('windows') else 'tar.gz'}"
        new_archive = current / name
        new_binary = (current / binary_path).read_bytes()
        old_path = binary_path.replace("darwin-native_", "jobs-cli_", 1)
        old_binary = (baseline / old_path).read_bytes()
        assert executable(new_archive) == new_binary, name
        old_size = "n/a (new target)"
        archive_delta = "n/a"
        if name in old_checksums:
            old_archive = baseline / name
            assert executable(old_archive) == old_binary, name
            old_size = old_archive.stat().st_size
            archive_delta = delta(old_size, new_archive.stat().st_size)
        rows.append(
            f"| {target} | {len(old_binary)} | {len(new_binary)} | "
            f"{delta(len(old_binary), len(new_binary))} | {old_size} | "
            f"{new_archive.stat().st_size} | {archive_delta} |"
        )
    casks = list(current.glob("**/jobs-cli.rb"))
    assert len(casks) == 1, casks
    cask = casks[0].read_text()
    assert "jobs-cli_darwin_universal.tar.gz" in cask, cask
    assert new_checksums["jobs-cli_darwin_universal.tar.gz"] in cask, cask
    for target in ("linux_x86_64", "linux_arm64"):
        name = f"jobs-cli_{target}.tar.gz"
        assert name in cask and new_checksums[name] in cask, name
    for target in ("darwin_x86_64", "darwin_arm64"):
        assert f"jobs-cli_{target}.tar.gz" not in cask, target
    rows += ["", "Generated Homebrew cask selects Universal on macOS and both Linux archives, with verified checksums."]
    for label, directory in (("Baseline", baseline), ("Current", current)):
        rows += ["", f"{label} GoReleaser metadata:", "```json",
                 json.dumps(json.loads((directory / "metadata.json").read_text()), indent=2), "```"]
    rows += ["", "GoReleaser is pinned to 2.15.4; Go is selected from go.mod."]
    return "\n".join(rows) + "\n"


if __name__ == "__main__":
    baseline, current, output = map(Path, sys.argv[1:])
    output.write_text(report(baseline, current))
