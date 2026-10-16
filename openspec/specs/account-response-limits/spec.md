# account-response-limits Specification

## Purpose
Sind die Usage-Daten eines Accounts gedrosselt oder fehlen, nutzt tollgate die Rate-Limit-Header der echten Antworten als Ersatzquelle für die Limits pro Account.

## Requirements

### Requirement: Antwort-Limits pro Account
Das Backend SHALL validierte Wochenlimits aus tatsächlichen Antworten dem beim Start gespeicherten Account zuordnen, auch für Beispielaufrufe. Es SHALL die neuesten Messstände pro Account persistent sichern, ohne den Proxy auf DB oder HTTP warten zu lassen. Fehler des separaten Usage-Endpunkts SHALL vorhandene neuere Antwort-Limits nicht als unbekannt darstellen.
#### Scenario: Usage-Endpunkt gedrosselt
- **WHEN** der Usage-Endpunkt HTTP 429 liefert und anschließend eine Antwort gültige Wochenlimits trägt
- **THEN** zeigt die Account-Karte Verbrauch und Reset aus dieser Antwort und behält den Messstand nach Neustart
#### Scenario: Andere Accounts und alte Daten
- **WHEN** ein weiterer Account genutzt wird oder ältere Antwort-Limits eintreffen
- **THEN** bleiben Messstände nach Account getrennt und ein älterer Messstand überschreibt keinen neueren
