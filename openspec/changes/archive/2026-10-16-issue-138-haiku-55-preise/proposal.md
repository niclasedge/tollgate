# Proposal

Issue: [#138 Meter: Preise für Claude Haiku 5.5 ergänzen, Cache-Read von Sonnet 5.5 korrigieren](https://github.com/niclasedge/tollgate/issues/138)

## Why

Seit #137 läuft `claude-haiku-5-5` über tollgate, aber der Meter kennt keinen Preis dafür. Kosten und 5-h-Einschätzung zählen das Modell mit $0. Außerdem rechnet der Cache-Read von `claude-sonnet-5-5` mit $0.20 statt $0.10 pro MTok.

## What Changes

- Neuer Preis für `claude-haiku-5-5` (Prompts bis 100K Tokens): Input $0.10, Output $0.50, Cache-Read $0.01, Cache-Write 5 m $0.125.
- Cache-Read von `claude-sonnet-5-5` sinkt auf $0.10 (0.05× Input).
- Preisstand im Code: 2026-10-09.
- Die höhere Stufe von Haiku 5.5 ab 100K Tokens fehlt bewusst. Berichte summieren die Usage vor der Bewertung, deshalb sind die Haiku-Kosten eine Untergrenze.

## Impact

- `internal/meter/cost.go`, Tests in `internal/meter/meter_test.go`.
- Kostenberichte für Sonnet 5.5 sinken rückwirkend etwas, weil sie aus der gespeicherten Usage neu berechnet werden.
