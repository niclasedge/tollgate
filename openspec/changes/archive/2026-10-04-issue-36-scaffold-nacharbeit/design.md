# Design

## Decisions

- **D1 Netzwerkfreiheit über den Abhängigkeitsgraphen:** `go list -deps` des Binarys darf weder `net`, `net/http`, `net/rpc` noch `crypto/tls` enthalten (reine Datenpakete wie `net/netip` bleiben erlaubt). Ohne diese Pakete kann das Binary weder Listener noch Verbindungen öffnen; das ist strenger und stabiler als ein Laufzeittest mit Port-Beobachtung. Sobald #6 den Proxy einführt, ersetzt #6 diesen Test durch einen Bindungstest.
- **D2 Ein Starttest:** `contract_test.go` prüft den Startvertrag strenger (genau eine Zeile, `make build` aus frischem Checkout); der doppelte Test in `main_test.go` entfällt.
- **D3 `make lint` statt einzelner CI-Befehle:** Lokal und in CI derselbe Befehl. `gofmt` erhält die Dateiliste aus `go list ./...` statt Verzeichnissen, damit eingebettete Module wie `third_party/cliproxyapi` nicht geprüft werden und eine leere Liste nicht auf stdin wartet.
