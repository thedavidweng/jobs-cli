# doctor and sources status are separate commands

`doctor` inspects local installation, config, session files, and optional connectivity. `sources status` reports discovery Sources and Application Providers with their capabilities (auth required, browser required, native submit, verification posture). Merging them would either bury the capability matrix or overload doctor with product surface area.

Supplement (issue #18): the Application Provider capability flags that `sources status` reports are derived from the canonical table in `internal/domain` (`CapabilitiesFor`) — the same table `resolve`, `show --resolve`, and `apply inspect` use — so the two surfaces cannot drift; a consistency test enforces it. `sources status` keeps verification posture and notes as its own presentation layer.
