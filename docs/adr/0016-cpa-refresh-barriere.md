# ADR-0016: Kleine CPA-Erweiterung als Refresh-Barriere

- Status: Angenommen im Rahmen des expliziten Auftrags zum zuverlässigen Account-Wechsel
- Issue: #77 (Reviewbefund aus #28)
- Branch: issue-77/cpa-refresh-barriere
- Ersetzt die Aussage „CPA bleibt ungepatcht“ aus ADR-0014 für diese begrenzte Erweiterung

## Begründung
Der gepinnte CPA erneuert Tokens unabhängig von tollgate. Download und Delete unter einer
Proxy-Sperre allein können daher ein verbrauchtes Refresh-Token sichern. Disabled stoppt
Refreshes ausdrücklich nicht (Upstream-Test `TestRefreshAuthForRequest_NormalDisabledAuth_RefreshesTokenSuccessfully`).
Wartezeiten ohne echte Synchronisation sind kein belastbarer Schutz.

## Entscheidung
Der Pin bleibt ausschließlich in `ARG CPA_VERSION`. Quellen werden per Go-Modul-Proxy
mit Prüfsumme bezogen und nur im temporären Build-Verzeichnis erweitert. Kein Fork,
Submodul oder Upstream-Code im Repository. Die MIT-Lizenz wurde vor der Erweiterung geprüft;
sie und die unveränderten Copyright-Hinweise bleiben im Image und in NOTICE.md erhalten.

Drei selbst geschriebene Zusatzdateien (Auth-Barriere, authentifizierter Management-Handler,
Regressionstest) und zwei kontrollierte Einfügestellen bilden die Erweiterung.
`bridge/apply.sh` bricht ab, wenn eine Einfügestelle nicht genau einmal vorkommt.
Der neue POST-Endpunkt `/v8/management/credentials/refresh-pause` übernimmt dieselbe
Management-Authentifizierung und Loopback-Bindung wie alle Credential-Routen.

Die Pause nimmt den vorhandenen Mutex pro Credential, wartet damit laufende Refreshes
inklusive Speicherung ab, schreibt den neuesten RAM-Stand explizit nochmals auf Disk
und setzt unter dem Mutex die Sperre. Jeder weitere Refresh prüft die Sperre nach
Erwerb desselben Mutex und vor jedem Netzaufruf. Der Unlock gibt den gesicherten Zustand
frei, während die Sperre bis Resume bestehen bleibt. tollgate sperrt parallel alle neuen
Inference-Requests und lehnt einen Wechsel bei laufenden Streams ab. Erst nach der
Pause wird heruntergeladen, verschlüsselt gesichert und getauscht. Der Ziel-Datei-ID wird
vor Upload ebenfalls eine Pause reserviert, damit dessen Registrierung keinen Refresh
vor der Verifikation starten kann. Bei Uploadfehlern wird auch eine eventuell geschriebene,
noch nicht registrierte Datei vor dem Rollback entfernt. Verifikation geht vor Resume;
ein mehrdeutiger Resume-Fehler nach erfolgreicher Verifikation führt nicht zur Rückabwicklung. Rollback, normale
Aktivierung und Übernahme nach tollgate-Neustart lösen die Pause des aktiven Accounts.
Bei Start wird Resume vor dem Listener versucht; bei noch nicht verfügbarem CPA wird es
vor jeder Proxy-Weiterleitung erneut geprüft, ohne eine Admin-Interaktion vorauszusetzen.
Ein fehlender Pause-Endpunkt führt vor jeder Token-Mutation zum sicheren Abbruch.

Die Pause selbst rotiert keinen Token und verändert keine Signatur oder Payload. Sie
ist RAM-Zustand: ein CPA-Neustart hebt sie auf; ein tollgate-Neustart nimmt den aktiven
Account wieder auf. Unterbrechung zwischen Delete und Upload: Wiederaktivierung aus
Tresor. Externe gleichzeitige CPA-Administration bleibt nicht unterstützt.

## Update und Nachweis
Herkunft und begründete Änderung: `third_party/cpa-refresh-barrier/PATCHES.md`.
Bei Pin-Updates immer die Barriere mit dem CPA-Regressionstest unter Race Detector
prüfen. Der Container-Build führt diesen Test aus; der Go-Gateway-Testbefehl bleibt
unverändert. Lokales `make cpa` baut weiterhin den reinen Upstream für dessen Baseline-Smoke.
