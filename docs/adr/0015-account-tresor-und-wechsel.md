# ADR-0015: Account-Tresor und manueller Wechsel

- Status: Angenommen durch expliziten Umsetzungsauftrag am 2026-10-04
- Issue: #28
- Branch: issue-28/account-wechsel

## Entscheidung
Alle Claude-Token-Dateien liegen AES-256-GCM-verschlüsselt in SQLite (`accounts`), mit der Account-ID als AAD. Der Schlüssel liegt unter `crypto.key_file`, standardmäßig neben der DB, Rechte 0600. Schlüssel und DB gemeinsam sichern. Eine fehlende Schlüsseldatei bei vorhandenem Tresor darf nicht automatisch ersetzt werden.

tollgate führt den PKCE-Code-Austausch selbst durch. Der CPA-Management-Login schreibt neue Tokens sofort in sein auth-dir und würde die Ein-Datei-Invariante verletzen. Keine Bibliotheksimporte. Die nach Review notwendige, begrenzte Refresh-Barriere wird in ADR-0016 separat begründet und mit #77 umgesetzt. Pro Login zufälliger State und Verifier, zehn Minuten TTL, einmaliger Austausch, höchstens zehn offene Logins. Der Benutzer kopiert die vollständige localhost-Callback-URL zurück ins Frontend; ein localhost-Listener ist nicht erforderlich. Netzwerkziele und Redirect-URI sind fest, Callback-URLs werden niemals abgerufen.

Protokollbeleg: gepinnte CPA-Quelle `internal/auth/claude/anthropic_auth.go`, https://github.com/router-for-me/CLIProxyAPI/blob/8ef43e4df3b216a42493105d31c2873b69191473/internal/auth/claude/anthropic_auth.go. OAuth-Token-Austausch nutzt platform.claude.com; Autorisierung claude.ai; Redirect localhost:54545/callback. Token-Format gemäß `internal/auth/claude/token.go`. Kein Code wird kopiert oder portiert.

Der Wechsel blockiert neue Proxy-Anfragen kurz mit einem wiederholbaren 503. Falls bereits eine Anfrage läuft, wird vor jeder Mutation abgebrochen. Der aktive Token wird unmittelbar vor dem Entfernen heruntergeladen und verschlüsselt zurückgesichert; vor dem Snapshot wartet die CPA-Refresh-Barriere (ADR-0016) laufende Rotation ab; danach Delete, Upload, Polling bis exakt der gewünschte aktive Account sichtbar ist. Fehler führen zu Wiederherstellung mit eigenem Timeout auch nach Request-Abbruch. Audit speichert Zeitpunkt, Account-ID und Ergebnis, niemals Token oder Payload. Absturz zwischen Delete und Upload: der Tresor hält beide Dateien; erneutes Aktivieren stellt einen Account wieder her. Externe gleichzeitige CPA-Administration ist nicht unterstützt; bei mehreren Dateien wird sicher abgebrochen.

Nach erfolgreicher Aktivierung läuft ein kurzer fester `/v1/messages`-Aufruf mit dem ausdrücklich gewünschten Modell `claude-sonnet-5-5` gegen CPA; das Frontend zeigt nur Status, Modell und feste Erfolg-/Fehlermeldung. Der Account bleibt auch bei einem fehlgeschlagenen Test aktiv (z. B. Rate-Limit), die Anzeige unterscheidet Wechsel und Test. Beim Wechsel wird die globale alte Limit-Anzeige gelöscht.

## Spike und Grenzen
Der Pin belegt `/api/oauth/profile` mit Account-/Organisationsidentität, aber keine verlässliche Zuordnung zu Pro/Max/Team und keine Garantie zur Lebensdauer inaktiver Refresh-Tokens. Diese Werte werden nicht geraten. Nutzungslimits stammen zunächst aus Antwort-Headern (bestehende Messung). Umfangreiche Lizenzabfragen und Limits pro inaktivem Account bleiben dem Folgechange #29 vorbehalten. Das Token-Format enthält Ablaufzeit und letzten Refresh; ein abgelaufenes Access-Token allein bedeutet nicht ungültiges Abo, weil CPA es erneuern kann.
