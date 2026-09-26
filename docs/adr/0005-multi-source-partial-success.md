# Multi-source partial success

When search fans out to multiple Sources, a failed Source does not fail the whole command if at least one Source succeeds: successful partitions return jobs/pagination as usual, failed partitions carry a structured per-partition error, and `meta.warnings` records the failure. The command exits non-zero only when every requested Source fails. This preserves Agent pipelines without pretending a partial outage is total success or total failure.
