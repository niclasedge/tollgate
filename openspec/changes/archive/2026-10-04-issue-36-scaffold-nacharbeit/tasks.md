# Tasks

- [x] 1.1 `main_test.go`: Test „kein `net` im Abhängigkeitsgraphen“ statt doppeltem Starttest. Prüfen: Mutation mit `_ "net/http"` in `main.go` macht den Test rot, sauber grün.
- [x] 1.2 `make lint` (gofmt + go vet) und CI-Schritt „Formatierung und vet“. Prüfen: absichtlich unformatierte Datei lässt `make lint` scheitern, sauber Exit 0.
- [x] 1.3 Makefile-Kommentare auf Englisch.
- [x] 1.4 PR mit `Closes #36`, CI grün, Merge, Archivierung.
