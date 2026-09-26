# doctor and sources status are separate commands

`doctor` inspects local installation, config, session files, and optional connectivity. `sources status` reports discovery Sources and Application Providers with their capabilities (auth required, browser required, native submit, verification posture). Merging them would either bury the capability matrix or overload doctor with product surface area.
