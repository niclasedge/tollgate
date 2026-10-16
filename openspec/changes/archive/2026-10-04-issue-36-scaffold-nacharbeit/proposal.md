# Proposal

Issue: [#36 Scaffold-Nacharbeit: Netzwerkfreiheit testen, doppelten Starttest zusammenführen, vet/gofmt in CI](https://github.com/niclasedge/tollgate/issues/36)

## Why

Das zweite Review von #2 fand drei Lücken: Die Spec verlangt, dass das Gerüst keine Netzwerkverbindung und keinen Listener öffnet, aber kein Test prüft das. Der Startvertrag wird in zwei Tests doppelt geprüft. Formatierung und `go vet` laufen in der CI nicht.

## What Changes

- `cmd/tollgate/main_test.go` prüft statt des doppelten Starttests, dass keine Netzwerkpakete (`net`, `net/http`, `net/rpc`, `crypto/tls`) im Abhängigkeitsgraphen des Binarys liegen. Der Startvertrag bleibt allein in `contract_test.go`.
- Neues Make-Ziel `make lint`: scheitert bei nicht formatierten Go-Dateien oder `go vet`-Befunden. Die CI führt es vor den Tests aus.
- Makefile-Kommentare auf Englisch (Projektkonvention: Code und Kommentare EN).

## Capabilities

### New Capabilities
Keine.

### Modified Capabilities
- `repo-scaffold`: neue Anforderung „Statische Prüfung“.

## Impact

- **Dateien:** `cmd/tollgate/main_test.go`, `Makefile`, `.github/workflows/ci.yml`.
- Ein-Weg-Tür: Projekt-Testbefehl und CI ändern sich (nur ergänzend).

## Was ausdrücklich nicht

Keine Änderung am Programmverhalten, keine neuen Abhängigkeiten.

## Berührte Dateien

- `cmd/tollgate/main_test.go`
- `Makefile`
- `.github/workflows/ci.yml`
