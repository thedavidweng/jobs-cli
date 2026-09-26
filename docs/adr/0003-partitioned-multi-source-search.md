# Multi-source search returns partitioned results

When `jobs-cli search` queries more than one Source, the public result is partitioned by Source (each partition with its own jobs and pagination), not a deduplicated flat list. Cross-source ranking and fingerprint dedup are Agent concerns; the CLI must not invent a shared ordering or silently drop duplicate openings that share an Application Provider.
