# Design

## Routing ohne ServeMux

`Server.ServeHTTP` verteilt mit festen Pfadregeln statt mit `http.ServeMux`. Grund: der ServeMux leitet unsaubere Pfade (`//v1/…`, `/a/../b`) per 301 um und würde damit verändern, was beim Upstream ankommt. Lokal sind nur:

| Pfad | Methoden | Ziel |
|---|---|---|
| `/api/clients`, `/api/clients/…` | alle | Admin-API (eigene Admin-Token-Prüfung) |
| `/`, `/index.html`, `/assets/…` | GET, HEAD | Frontend |
| alles andere | alle | Proxy (Pipeline, Forward) |

`/api/clientsX` oder `/api/other` gehen an den Proxy. Ohne `Frontend`/`Admin` in den Optionen verhält sich der Server wie in #6. Claude Code ruft `/` nie auf, deshalb kollidiert die Frontend-Route nicht mit dem Durchreichen.

## Admin-Token

`auth.LoadOrCreateAdminToken` (aus #8) liest die Datei oder erzeugt sie mit 0600. Der Default neben `db.path` macht den ersten Start ohne zusätzliche Konfiguration möglich; ADR-0008 nennt `admin.token_file` bereits. Der Pfad (nicht der Token) steht in der Logzeile `tollgate: listening`.

## Generierte Typen

`tygo` liest nur `internal/auth/types.go` (`include_files`), damit `Verifier` und Hilfstypen nicht als TS-Interfaces erscheinen. `rotated_at` trägt `tstype:"string | null,required"`, weil Go `null` sendet und das Feld nie fehlt.
