# Design

- **Zweimal prüfen statt Vertrauen weiterreichen.** Der Server prüft vor dem Body-Read, `Intake.Process` prüft im Runner erneut. Die zweite Prüfung kostet einen SHA-256 und einen indizierten SQLite-Lookup. Dafür bleibt die Stufe eigenständig, und es gibt keinen Weg, eine Client-ID ungeprüft in den Request zu setzen. Ein Key, der zwischen beiden Prüfungen gesperrt wird, fällt bei der zweiten durch.
- **Interface statt Typprüfung.** `Runner.Authenticate` sucht die erste Stufe, die `Authenticator` implementiert. Ein Runner ohne solche Stufe (Tests) lässt alles durch, wie bisher `Run` ohne Intake.
- **Kein Trace.** Unauthentifizierte Anfragen hinterließen schon vorher keinen Trace (keine Client-ID). Das bleibt so.
