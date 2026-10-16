# limit-alerts Specification

## Purpose
TBD - created by archiving change issue-122-limit-alarm. Update Purpose after archive.

## Requirements

### Requirement: Warnung vor dem 5-Stunden-Limit

tollgate SHALL für jede Antwort mit Limit-Headern die Auslastung des 5-Stunden-Fensters als ganze Prozentzahl `floor(utilization · 100)` bestimmen. Ab der Schwelle `alerts.five_hour_threshold` (Default 90) MUST je Abo und Fenster genau eine Nachricht je höherem vollem Prozentpunkt entstehen; gleiche oder niedrigere Werte im selben Fenster MUST keine Nachricht erzeugen. Ein anderer Reset-Zeitpunkt des Fensters MUST den Zähler zurücksetzen. Die Nachricht MUST den Namen des Abos, die Prozentzahl, die Zeit bis zum Reset und den besten Ausweich-Kandidaten nennen oder ausdrücklich sagen, dass keiner frei ist. Die Beobachtung MUST den Proxy nicht blockieren.

#### Scenario: Ein Prozentpunkt, eine Nachricht
- **WHEN** nacheinander Antworten mit 89 %, 90 %, 90,4 %, 91 % und 90 % im selben Fenster eintreffen
- **THEN** entstehen genau zwei Nachrichten, für 90 % und 91 %.

#### Scenario: Neues Fenster
- **WHEN** nach einer Nachricht für 95 % eine Antwort mit 92 % und anderem Reset eintrifft
- **THEN** entsteht eine Nachricht für 92 %.

#### Scenario: Sprung über mehrere Punkte
- **WHEN** nach 90 % direkt 93 % gemeldet werden
- **THEN** entsteht genau eine Nachricht für 93 %.

### Requirement: Ausweich-Kandidat

tollgate SHALL als Kandidaten nur Abos wählen, die nicht aktiv sind und nicht das betroffene Abo sind, deren Wochenlimit unter 100 % liegt oder unbekannt ist und deren 5-Stunden-Fenster unter 100 % liegt oder unbekannt ist. Ein Fenster mit vergangenem Reset MUST als frei gelten. Abos, die laut Lizenzstand eine neue Anmeldung brauchen, MUST ausgeschlossen sein. Die Rangfolge MUST deterministisch sein: bekanntes Wochenlimit vor unbekanntem, dann niedrigere 5-Stunden-Auslastung (unbekannt zählt als 0), dann niedrigere Wochenauslastung, dann Name und ID.

#### Scenario: Volles Wochenlimit
- **WHEN** Abo B 100 % Woche und Abo C 40 % Woche bei unbekanntem 5-Stunden-Fenster hat
- **THEN** ist C der Kandidat.

#### Scenario: Abgelaufenes Fenster
- **WHEN** Abo B im 5-Stunden-Fenster 100 % mit einem Reset in der Vergangenheit hat
- **THEN** gilt sein 5-Stunden-Fenster als frei.

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

### Requirement: Zustellung über Spool und Notifier

tollgate SHALL Nachrichten atomar (temporäre Datei, dann Umbenennen) als JSON-Dateien in `alerts.spool_dir` schreiben; ohne Spool-Verzeichnis MUST keine Nachricht entstehen, der Wechsel funktioniert trotzdem. Der Befehl `tollgate notify` SHALL das Verzeichnis in festen Abständen in Namensreihenfolge abarbeiten, jede Nachricht per `chat.postMessage` senden und erst nach `ok: true` löschen. Bei einem Fehler MUST er abbrechen und es später erneut versuchen; Dateien älter als 24 Stunden und unlesbare Dateien MUST er verwerfen. Ist die Token-Datei leer oder fehlt sie, MUST er Nachrichten ohne Senden verwerfen. Token und Kanal MUST er bei jedem Durchlauf neu lesen. Fehlermeldungen und Logs MUST den Token nie enthalten. tollgate selbst MUST keinen Zugriff auf den Slack-Token haben, und die Egress-Sperre aus ADR-0017 MUST unverändert bleiben.

#### Scenario: Erfolgreiche Zustellung
- **WHEN** zwei Dateien im Spool liegen und Slack `{"ok":true}` antwortet
- **THEN** gehen beide in Namensreihenfolge raus und werden gelöscht.

#### Scenario: Slack-Fehler
- **WHEN** Slack `{"ok":false,"error":"channel_not_found"}` antwortet
- **THEN** bleibt die Datei liegen, und der Fehler nennt `channel_not_found`, nicht den Token.

#### Scenario: Slack aus
- **WHEN** die Token-Datei leer ist
- **THEN** löscht der Notifier die Dateien ohne Anfrage an Slack.
