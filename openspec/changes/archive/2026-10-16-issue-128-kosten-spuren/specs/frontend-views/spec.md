## ADDED Requirements

### Requirement: Kostenkurve mit einer Linie je Account

Die Kostenkurve SHALL je Account eine eigene Linie in einem gemeinsamen Diagramm auf gemeinsamer Skala zeigen. Die Linien MUST übereinander gezeichnet und MUST NOT gestapelt werden, sodass jede Linie die Kosten ihres Accounts zeigt. Die ersten fünf Accounts SHALL in fester Reihenfolge je eine eigene kategoriale Farbe bekommen, die in hellem und dunklem Modus für Farbsehschwäche geprüft ist; weitere Accounts MUST grau erscheinen statt eine Farbe zu wiederholen. Legenden der Accounts MUST dieselben Farben verwenden.

#### Scenario: Zwei Accounts mit gleichen Kosten
- **WHEN** zwei Accounts in derselben Stunde je $1 kosten
- **THEN** liegen beide Linien in dieser Stunde auf der Höhe von $1, in zwei Farben.
