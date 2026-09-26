# Multi-source search fans out in parallel

When `search` queries multiple Sources, provider requests run concurrently under the invocation `--timeout`. Partial-success rules still apply per Search Partition. Parallelism is the default because Sources are independent network backends; serial execution is not required for correctness.
