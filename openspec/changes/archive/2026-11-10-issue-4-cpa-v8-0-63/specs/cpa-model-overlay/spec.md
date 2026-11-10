## REMOVED Requirements

### Requirement: Fehlende Modelle im CPA-Katalog
**Reason**: Der Pin v8.0.63 bettet `claude-haiku-5-5` und `claude-fable-5-2` ein; das Build-Overlay ist überflüssig (ADR-0022).
**Migration**: Neue Modelle kommen über ein Pin-Update nach `deploy/cliproxyapi/UPSTREAM.md`.
