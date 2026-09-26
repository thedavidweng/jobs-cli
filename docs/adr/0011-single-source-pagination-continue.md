# Search pagination continues one Source at a time

The first `search` may fan out to multiple Sources and return Search Partitions. Continuation requests must name a single Source plus that Source’s native continuation token (cursor/offset). There is no opaque global page token and no cross-source “next page,” which would invent a shared pagination model the backends do not share.
