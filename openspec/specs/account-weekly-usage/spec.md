# account-weekly-usage Specification

## Purpose
Pro Abo zeigt tollgate die Wochenlimits mit Zeitfortschritt und Reset-Zeitpunkt, damit ein Wechsel rechtzeitig geplant werden kann.

## Requirements

### Requirement: Usage pro Account
Das Backend SHALL für jeden gespeicherten Account Usage über einen festen HTTPS-Endpunkt abfragen, ohne den aktiven Account oder Token-Dateien zu wechseln. Es SHALL nur validierte Limit-Werte speichern und ausgeben, erfolgreiche Reads fünf Minuten cachen und fehlgeschlagene Reads begrenzt wiederholen. Tokens und Upstream-Fehlertexte SHALL nie in API, Logs oder DB gelangen.
#### Scenario: Inaktiver Account
- **WHEN** Usage eines inaktiven Accounts abgerufen wird
- **THEN** wird dessen verschlüsseltes Access-Token nur im RAM gelesen, und im auth-dir bleibt nur der aktive Account
#### Scenario: Rotiertes aktives Token
- **WHEN** Usage des aktiven Accounts abgerufen wird
- **THEN** verwendet der Read das aktuelle CPA-Access-Token und nicht einen alten Tresorstand
#### Scenario: Fehler oder Neustart
- **WHEN** ein Read fehlschlägt oder tollgate neu startet
- **THEN** bleibt der letzte erfolgreiche Messstand erhalten und der Fehler wird ohne Upstream-Inhalte angezeigt

### Requirement: Wochenvergleich
Das Frontend SHALL je Account Verbrauch, verbleibenden Anteil, Reset, Messzeit und die Zeitposition in der sieben Tage langen Periode zeigen. Es SHALL unbekannte, veraltete und abgelaufene Fenster kenntlich machen.
#### Scenario: Verbrauch und Zeit
- **WHEN** ein gültiges Wochenfenster vorliegt
- **THEN** zeigt die Karte den Verbrauch und eine getrennte Jetzt-Markierung berechnet aus Reset minus sieben Tage
#### Scenario: Reset überschritten oder Daten fehlen
- **WHEN** das Reset-Datum vorbei ist oder kein Messwert vorliegt
- **THEN** behauptet die Karte kein freies Budget und zeigt den Bedarf an einem neuen Messstand
