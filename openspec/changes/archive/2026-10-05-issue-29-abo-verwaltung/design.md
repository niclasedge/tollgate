## Bestand (bereits gemergt)
| Issue-Punkt | Stand | Quelle |
|---|---|---|
| Liste mit Name, E-Mail, aktiv, runder Indikator | vorhanden | #28 `AccountManager.svelte` |
| Import einer Token-Datei, Inhalt nie angezeigt | vorhanden | #28 |
| Abo hinzufügen per Login-Link und Callback-URL | vorhanden | #28 |
| Aktivieren mit Inline-Bestätigung, Hinweis Prompt-Cache | vorhanden | #28 |
| Fortschritt, Ergebnis, Rollback-Meldung | vorhanden | #28, #88 |
| Wochenlimit als Balken mit Reset und Zeitposition | vorhanden | #78 |
| Token gültig / neu einloggen nötig | fehlt (nur Fehlertext bei Usage-401) | dieser Change |
| Plan und Status | fehlt | dieser Change |
| 5 Stunden als Balken | fehlt (nur Text) | dieser Change |
| „Stand vom …“ für inaktive Abos | nur „Stand: …“ | dieser Change |
| dieselbe Karte je Abo auf der Startseite | fehlt (#76 verweist auf #29) | dieser Change |
| Escape bricht Inline-Bestätigung ab | fehlt | dieser Change |

## Entscheidungen
**Profil-Read im Usage-Read.** Der Profil-Read läuft in `fetchSubscriptionUsage`, also unter derselben Fünf-Minuten-Drosselung, demselben Token-Snapshot (aktiv: aktuelles CPA-Token, inaktiv: Tresor im RAM) und derselben Persistenz wie #78. Alternative eigener Endpunkt mit eigener Tabelle: doppelte Drossel-/Cache-Logik und eine Migration ohne Mehrwert. Tests, die `fetchUsage` ersetzen, rufen dadurch nie das echte Netz auf.

**Nur feste Bezeichner.** Aus `organization` werden `organization_type`, `rate_limit_tier` und optional `subscription_status` übernommen, jeweils nur bei `^[a-z0-9_]{1,64}$`. Namen, UUIDs, E-Mail und alle übrigen Felder werden verworfen. Fehlt `subscription_status` im Profil, bleibt der Status leer und das Frontend zeigt den CPA-Status des aktiven Abos.

**Token-Zustand.** `valid`, wenn Usage- oder Profil-Read 200 liefert. Liefert der Read 401: `expired`, wenn das Ablaufdatum der Token-Datei (`expired`, RFC 3339) vorbei ist (CPA erneuert beim Aktivieren bzw. vor der nächsten Anfrage), sonst `relogin`. Ohne lesbares Ablaufdatum oder bei anderen Fehlern `unknown`. tollgate refresht inaktive Tokens nicht selbst, weil das Refresh-Tokens rotieren würde.

**Persistenz.** `UsageSnapshot.license` liegt im vorhandenen JSON-Messstand (`account_usage`). Nach jedem Read-Versuch werden Token-Zustand und Prüfzeit gespeichert; Plan-Felder nur nach erfolgreichem Profil-Read, sonst bleibt der letzte Plan mit seiner eigenen Messzeit. Beobachtungen aus Antwort-Headern (#85/#86) überschreiben die Lizenz nie.

**Eine Karte.** `AccountCard` kapselt Kopf, Lizenzzeile und `AccountUsage`; Aktionen kommen als Snippet. Accounts und Startseite nutzen sie, die Startseite ohne Aktionen. Ohne Account-Verwaltung bleibt die bisherige `LimitCard`.

**Plan-Anzeige.** `claude_max` → „Max“, `claude_pro` → „Pro“, `claude_team` → „Team“, `claude_enterprise` → „Enterprise“; Stufen `*_5x`/`*_20x` als Zusatz „5x“/„20x“. Unbekannte Werte erscheinen unverändert, fehlende als „Plan unbekannt“.

## Risiken
Die Profilfelder sind nicht live gegen ein echtes Abo geprüft (kein Zugriff auf echte Accounts in diesem Change). Fehlende oder anders benannte Felder führen zu „Plan unbekannt“, nie zu geratenen Werten.
