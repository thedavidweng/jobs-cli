# Native Linux amd64 UPX experiment

Decision: **REJECT; keep UPX disabled in production and experimental release assets.**

Measured on 2026-10-18 (America/Vancouver) using native SSH host `owo`, Linux-6.12.101+deb13-amd64-x86_64-with-glibc2.41. No system packages, global cache settings, releases or Homebrew tap files were changed.

## Inputs

- PR #36 Packaging run [39106479629](https://github.com/thedavidweng/jobs-cli/actions/runs/39106479629), `release-snapshot` artifact `12128926768`.
- Snapshot source commit `1ab52eed20876e52667a7f49c52b8f38c24d4f2b`, version `2.1.2-next`, Go `go1.27.1`. This is an existing CI snapshot, not a rebuild of subsequent PR heads.
- Original CI `jobs-cli_linux_x86_64.tar.gz`: **6,966,412 bytes**; checked against its supplied SHA-256 manifest.
- Official UPX 5.2.1 amd64 Linux archive SHA-256: `402162aad30af47e60dbd767fb2e64ca394ace9727ba1f40283641f1d1b91657`.
- Candidate command: `upx --best --lzma packed/jobs-cli`; `upx -t packed/jobs-cli` passed.

## Size

| Measurement | Original bytes | UPX bytes | Delta |
| --- | ---: | ---: | ---: |
| Executable | 15,114,400 | 5,766,528 | -61.85% |
| Matched tar.gz (gzip level 9) | 6,868,237 | 5,779,238 | -15.86% |

Matched archives preserve entry order and tar metadata and use identical gzip settings/mtime; only the executable payload differs. The recompressed control differs from the original CI archive because its compressor/settings differ. The matched pair is the compression comparison.

## Startup and peak RSS

Each cell is the median of 21 alternating original/candidate samples. Time includes fork/exec, program startup and exit; a small native C parent uses `CLOCK_MONOTONIC` and `wait4.ru_maxrss` so the Python orchestrator's memory high-water mark cannot contaminate RSS. Output is redirected to `/dev/null` during timing.

| Command | Mode | Original ms | UPX ms | Original peak RSS KiB | UPX peak RSS KiB |
| --- | --- | ---: | ---: | ---: | ---: |
| `--help` | warm | 8.517 | 373.778 | 15,284 | 20,720 |
| `--help` | cold-file | 8.718 | 368.145 | 15,296 | 20,728 |
| `--version` | warm | 8.314 | 370.634 | 15,236 | 20,732 |
| `--version` | cold-file | 8.775 | 378.833 | 15,260 | 20,660 |
| `sources status --json` | warm | 8.978 | 374.743 | 15,300 | 20,736 |
| `sources status --json` | cold-file | 8.817 | 369.781 | 15,320 | 20,752 |
| `not-a-command` | warm | 8.519 | 364.977 | 15,300 | 20,720 |
| `not-a-command` | cold-file | 8.240 | 370.895 | 15,288 | 20,712 |

Cold-file means `posix_fadvise(POSIX_FADV_DONTNEED)` on the executable before each sample. This is a file-cache eviction request, not proof of a globally cold kernel/page cache. No global cache drop was performed on the shared host.

Exit status, stdout and stderr matched for all four commands, with only the per-invocation `sources status --json` `meta.request_id` UUID normalized. The normal command inspects local Source capability status without network access. Version/commit/build metadata remained intact.

UPX saves about 15.86% on the matched compressed archive but increases startup from about 8–9 ms to 365–379 ms (over 40 times) and peak RSS by about 5.3 MiB. This exceeds the 10% startup regression rejection threshold and is sufficient to reject packing. Security-scanner compatibility, other Linux distributions and a full cold-boot experiment are not claimed or needed to override this rejection.

## Reproducibility

The coordination evidence directory `outputs/jobs-upx/` retains `benchmark.py`, `measure.c`, `results.json` (all 336 measured process samples), `benchmark.log`, original snapshot/checksums and the verified UPX archive. The isolated native workspace is `/tmp/jobs-upx.fJ42S6Sk` on `owo`.

```shell
cc -O2 -Wall -Wextra -o measure measure.c
python3 benchmark.py
```
