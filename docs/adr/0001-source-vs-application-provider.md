# Separate Source from Application Provider

Discovery origin and application execution are different domain concepts and must not share one “provider” field. A Job discovered on Indeed may apply on Greenhouse; treating them as one type collapses routing, capabilities, and auth requirements. Commands, IDs, and JSON contracts therefore carry `source` for provenance and `application.provider` (via an Application Target) for apply behavior.
