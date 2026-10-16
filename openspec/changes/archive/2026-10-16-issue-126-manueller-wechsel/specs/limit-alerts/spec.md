## MODIFIED Requirements

### Requirement: Automatischer Wechsel bei Ablehnung

Ist `alerts.auto_switch` an, SHALL tollgate nach einer abgelehnten Antwort des aktiven Abos asynchron auf den besten Kandidaten wechseln und das Ergebnis melden. Als abgelehnt MUST gelten: Header-Status `rejected` (gesamt, 5 Stunden oder Woche), oder HTTP 429, wenn die letzte Beobachtung dieses Abos abgelehnt war oder ein Fenster mindestens an der Warnschwelle lag. CLIProxyAPI reicht bei einer 429 die Limit-Header nicht weiter; ein 429 ohne diese Vorgeschichte MUST unbeachtet bleiben. Gibt es keinen Kandidaten oder ist das Abo nicht mehr aktiv, MUST kein Wechsel erfolgen; ohne Kandidaten MUST eine Meldung entstehen. Zwischen zwei Versuchen MUST mindestens 10 Minuten liegen, und es MUST höchstens ein Versuch gleichzeitig laufen. Ohne Angabe MUST `alerts.auto_switch` aus sein, im Binary wie in `docker-compose.yml`.

#### Scenario: Wechsel
- **WHEN** das aktive Abo A eine Antwort mit Status `rejected` erhält und C frei ist
- **THEN** aktiviert tollgate C und meldet „gewechselt auf C“.

#### Scenario: Cooldown
- **WHEN** fünf Minuten nach einem Wechselversuch erneut `rejected` gemeldet wird
- **THEN** erfolgt kein weiterer Versuch.

#### Scenario: 429 ohne Header nahe am Limit
- **WHEN** A zuletzt 97 % im 5-Stunden-Fenster meldete und danach HTTP 429 ohne Limit-Header kommt
- **THEN** gilt das als Ablehnung.

#### Scenario: 429 unter der Schwelle
- **WHEN** A zuletzt 89 % meldete und danach HTTP 429 kommt
- **THEN** wechselt tollgate nicht und meldet nichts.

#### Scenario: Wechsel abgeschaltet
- **WHEN** `alerts.auto_switch` aus ist und `rejected` gemeldet wird
- **THEN** wechselt tollgate nicht und meldet die Ablehnung mit Kandidat.

#### Scenario: Standard ohne Angabe
- **WHEN** weder Konfiguration noch `TOLLGATE_ALERTS_AUTO_SWITCH` den Wechsel einschalten und `rejected` gemeldet wird
- **THEN** wechselt tollgate nicht und meldet die Ablehnung mit Kandidat.
