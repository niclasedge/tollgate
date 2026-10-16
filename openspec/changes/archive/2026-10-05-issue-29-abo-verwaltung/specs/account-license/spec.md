## ADDED Requirements

### Requirement: Lizenzstatus je Abo
Das Backend SHALL beim gedrosselten Usage-Read jedes gespeicherten Abos zusätzlich `https://api.anthropic.com/api/oauth/profile` mit demselben, nur im RAM gehaltenen Token lesen, ohne den aktiven Account, das auth-dir oder Token-Dateien zu ändern. Es SHALL nur `organization_type`, `rate_limit_tier` und `subscription_status` übernehmen, jeweils nur als Bezeichner aus `[a-z0-9_]` mit höchstens 64 Zeichen. Upstream-Bodies, Namen, UUIDs und Tokens MUST NOT in API, Logs oder DB gelangen.

#### Scenario: Profil eines inaktiven Abos
- **WHEN** das Profil eines inaktiven Abos `organization_type: claude_max` und `rate_limit_tier: default_claude_max_20x` liefert
- **THEN** enthält `license` diese Werte samt Messzeit, und im auth-dir liegt weiter nur die Datei des aktiven Abos

#### Scenario: Unerwartete Werte
- **WHEN** das Profil Felder mit Leerzeichen, Großbuchstaben oder über 64 Zeichen oder zusätzliche Felder wie Namen enthält
- **THEN** bleiben diese Plan-Felder leer, und die Antwort enthält keinen dieser Werte

### Requirement: Token-Zustand
Das Backend SHALL je Read-Versuch einen Token-Zustand mit Prüfzeit speichern: `valid`, wenn Usage- oder Profil-Read erfolgreich ist; bei HTTP 401 `expired`, wenn das Ablaufdatum der Token-Datei vergangen ist, sonst `relogin`; in allen anderen Fällen `unknown`. Der Zustand und der letzte Plan MUST nach Neustart erhalten bleiben und MUST NOT durch Limit-Beobachtungen aus Antwort-Headern gelöscht werden.

#### Scenario: Abgewiesenes gültiges Token
- **WHEN** beide Reads mit 401 antworten und das Token laut Datei noch nicht abgelaufen ist
- **THEN** ist der Zustand `relogin`, und der zuvor gemessene Plan bleibt mit seiner Messzeit erhalten

#### Scenario: Abgelaufenes Access-Token
- **WHEN** beide Reads mit 401 antworten und das Ablaufdatum der Datei vorbei ist
- **THEN** ist der Zustand `expired`, nicht `relogin`

#### Scenario: Usage verweigert, Profil erreichbar
- **WHEN** der Usage-Read 403 und der Profil-Read 200 liefert
- **THEN** ist der Zustand `valid`, der Plan gesetzt und der Usage-Fehler weiterhin sichtbar

### Requirement: Abo-Karte
Das Frontend SHALL je Abo eine Karte mit Name, E-Mail, rundem Aktiv-Indikator, Token-Zustand („Token gültig“, „Zugang abgelaufen“, „Neu einloggen nötig“, „Token-Status unbekannt“), Plan und Status sowie 5-Stunden- und Wochenverbrauch je als dezentem Balken mit Reset zeigen. Für inaktive Abos MUST die Lizenz mit „Stand vom …“ datiert sein. Bei `relogin` eines inaktiven Abos SHALL die Karte „Neu anmelden“ anbieten. Escape MUST eine offene Inline-Bestätigung abbrechen.

#### Scenario: Wechsel von Abo 1 auf Abo 2
- **WHEN** der Nutzer bei Abo 2 „Aktivieren“ wählt, den Hinweis „Prompt-Cache startet neu“ sieht und bestätigt
- **THEN** zeigt die Seite Abo 2 als aktiv, und der Lizenzstatus beider Abos ist sichtbar

#### Scenario: Neu einloggen nötig
- **WHEN** ein inaktives Abo den Token-Zustand `relogin` hat
- **THEN** zeigt die Karte „Neu einloggen nötig“ mit Warn-Indikator und einen Knopf „Neu anmelden“, der den Login-Ablauf mit dem Namen des Abos startet

#### Scenario: Escape
- **WHEN** die Inline-Bestätigung offen ist und der Nutzer Escape drückt
- **THEN** schließt sie ohne Aufruf von `activate`, und der Fokus liegt auf „Aktivieren“
