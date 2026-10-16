# Tasks

## 1. Grundgerüst der Dokumente

- [x] 1.1 `docs/adr/README.md` anlegen: ADR-Vorlage (Status, Kontext, Entscheidung, Alternativen, Konsequenzen, Quellen) und Index von ADR-0001 bis ADR-0009. Prüfen: `ls docs/adr` zeigt den Index, jeder Indexeintrag verlinkt auf eine geplante Datei.

## 2. ADRs schreiben (Status „Vorgeschlagen“)

- [x] 2.1 ADR-0001 „CLIProxyAPI als eigener Prozess“ nach design.md D1. Inhalt: Commit `8ef43e4`, Lizenz, Pflichtkonfiguration, Header-Regeln, Rückfalloption gepinntes Image. Prüfen: Abschnitt „Alternativen“ enthält SDK-Einbettung und reines Image; jede Aussage zu CLIProxyAPI hat eine Pfadangabe.
- [x] 2.2 ADR-0002 „Zielstruktur des Repos“ nach D2, mit vollständigem Verzeichnisbaum. Prüfen: Jede Abweichung vom Vorschlag in `CLAUDE.md` ist begründet.
- [x] 2.3 ADR-0003 „Bibliotheken“ nach D3, Tabelle aus Wahl und Verworfenem. Prüfen: Ob `encoding/json/jsontext` in Go 1.26 ohne GOEXPERIMENT verfügbar ist, ist per `go doc encoding/json/jsontext` geklärt und das Ergebnis eingetragen.
- [x] 2.4 ADR-0004 „JSON per Byte-Splicing und Determinismus“ nach D4, mit Determinismus-Checkliste. Prüfen: Die harten Regeln „unbekannte Felder“ und „Byte für Byte“ sind explizit referenziert.
- [x] 2.5 ADR-0005 „Redactor-Port“ nach D5. Inhalt: Tabelle der Schalter, Liste 1:1 vs. neu, Vorgehen beim Paritätstest, Liste der bewussten Paritätsabweichungen. Prüfen: Jede Rust-Regel ist einem Schalter zugeordnet oder als „nicht portiert“ markiert.
- [x] 2.6 ADR-0006 „Platzhalter, Sitzungen und Rückweg“ nach D6. Inhalt: Grammatik, Nummerierung pro Anfrage, Blocktabelle Hin- und Rückweg, `count_tokens`, Upstream-Header. Prüfen: Thinking und Signaturen sind für beide Richtungen als unverändert festgelegt.
- [x] 2.7 ADR-0007 „Kompression und Headroom-Port“ nach D7, mit Listen „portiert“ und „nicht portiert“ samt Begründung. Prüfen: Jede portierte Strategie nennt ihre Quelldatei; die Apache-2.0-Pflichten (NOTICE, Änderungsvermerk) sind genannt.
- [x] 2.8 ADR-0008 „Konfigurationsschema“ nach D8. Inhalt: Bootstrap-Felder, YAML-Beispiel der Laufzeitkonfiguration, Import-Ablauf, Versionierung und Rollback. Prüfen: Das Beispiel aus `CLAUDE.md` bleibt ohne Änderung gültig, nur `compress:` kommt hinzu.
- [x] 2.9 ADR-0009 „Authentifizierung, Schlüssel, Geheimnisse“ nach D9. Prüfen: Die harten Regeln „nur redigierte Daten in Logs, DB und Frontend“ und „Bindung an Tailnet bzw. localhost“ sind jeweils einer Maßnahme zugeordnet.

## 3. Architekturdokument

- [x] 3.1 `docs/ARCHITECTURE.md` schreiben. Inhalt: Überblick, Datenfluss als Mermaid-Diagramm, Komponenten und Pakete, Pipeline-Stufen 0–5, Sitzungsmodell, Netzwerk und Prozesse. Prüfen: Jede Stufe nennt Eingabe, Ausgabe und zuständiges Paket.
- [x] 3.2 Matrix der harten Regeln in `ARCHITECTURE.md` (Regel → ADR → Durchsetzung → geplanter Test). Prüfen: Alle Punkte aus dem Abschnitt „Harte Regeln“ in `CLAUDE.md` sind enthalten.
- [x] 3.3 Abschnitt „Abweichungen vom Auftrag (zur Freigabe)“. Prüfen: Er enthält mindestens diese sieben Punkte, jeweils mit Begründung und Link zum ADR:
  - Assistant-Blöcke werden mitredigiert
  - Zuordnung pro Anfrage statt Sitzungs-TTL
  - `count_tokens` wird redigiert
  - kein Rehydrate von Thinking
  - `connection_urls` ersetzt nur das Passwort
  - Treffer werden erst gesammelt, dann aufgelöst
  - CLIProxyAPI-Kopie als eigenes Modul
- [x] 3.4 Abschnitt „Auswirkungen auf Folge-Issues“: pro betroffenem Issue (#2–#25) die nötige Anpassung in einem Satz. Prüfen: Mindestens #3, #5, #6, #12, #13, #14, #19 und #20 sind behandelt.

## 4. Prüfung und Freigabe

- [x] 4.1 Konsistenzprüfung: Keine Secrets, keine internen Hostnamen und keine Kundennamen in `docs/`; alle Links zwischen ADRs und Architektur funktionieren. Prüfen: `grep -rniE '<employer-name>|password=|sk-ant-|BEGIN .*PRIVATE' docs/` ist leer; jeder relative Link zeigt auf eine existierende Datei.
- [x] 4.2 `openspec validate issue-1-architektur` ist grün; PR mit `Closes #1` öffnen. Prüfen: PR-Link vorhanden.
- [x] 4.3 Freigabe durch Niclas einholen. Erst danach den Status der ADRs auf „Angenommen“ setzen. Prüfen: Die Freigabe ist im PR oder Issue #1 dokumentiert.
- [x] 4.4 Nach der Freigabe die Folge-Issues gemäß 3.4 mit `gh issue edit` anpassen. Prüfen: Jedes angepasste Issue verweist auf das passende ADR.
