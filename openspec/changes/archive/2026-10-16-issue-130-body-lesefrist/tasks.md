# Tasks

## 1. Umsetzung

- [x] 1.1 Tests zuerst (`bodyread_test.go`): langsamer, stetiger Sender über der Frist kommt durch; stiller Sender scheitert und wird ohne Inhalt geloggt. Gegenprobe ohne Fristverschiebung schlägt fehl (per `go test -overlay`).
- [x] 1.2 `idleBody` verschiebt die Lesefrist bei jedem Block und loggt den ersten Fehler; in `serveProxy` eingehängt.

## 2. Abschluss

- [x] 2.1 `make test`, `make lint` grün; deployt und über das Relay gemessen.
