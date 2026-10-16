# Gate 1 — issue-59-image-digests

- Datum: 2026-10-05T07:38:00Z
- Person: niclasedge (Subagent im Auftrag des Koordinators)
- Branch: issue-59/image-digests (noch nicht angewendet; der Koordinator legt ihn per `but commit -b` an)
- Pruefumfang: gemeinsamer Workspace aller angewendeten Branches; kein isolierter Branch-Nachweis
- Commit: c64ca81c292880c4223735782b975ca75f44389f
- Werkzeug: `just gate1` verweigert ohne angewendeten Branch; Pruefungen von Hand nachvollzogen. Nach dem Anlegen des Branches `just gate1 issue-59-image-digests issue-59/image-digests` erneut ausfuehren.

## Pruefungen

- bestanden — G1-1: keine Platzhalter der Vorlage mehr im Change
- bestanden — G1-2: proposal.md hat Warum, Aenderung, Abgrenzung und Beruehrte Dateien
- bestanden — G1-3: mindestens eine Delta-Spec mit Delta-Block (`specs/deployment/spec.md`, ADDED)
- bestanden — G1-4: jedes Requirement hat mindestens ein Szenario
- bestanden — G1-5: tasks.md enthaelt 1 bis 8 Aufgaben (5)
- bestanden — G1-6: tasks.md nennt die Testdatei `deploy/baseimages_test.go`
- bestanden — G1-7: `openspec validate issue-59-image-digests --strict` laeuft durch
- bestanden — G1-8/9: justfile unveraendert

## Belege

- Requirements 1, Szenarien 4, Delta-Bloecke 1
- Zugesagte Testdatei: deploy/baseimages_test.go
- Capabilities: deployment
