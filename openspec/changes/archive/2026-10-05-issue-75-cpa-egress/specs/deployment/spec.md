## ADDED Requirements

### Requirement: Egress des CPA-Namespaces nur zu Anthropic

Der Entrypoint des CLIProxyAPI-Images SHALL vor dem Start von CLIProxyAPI für IPv4 und IPv6 in der Kette `OUTPUT` der Tabelle `filter` zuerst die Policy `DROP` setzen und danach nur Ausgang über `lo`, Pakete im Zustand `ESTABLISHED,RELATED`, `tcp/443` zu `160.79.104.0/23` bzw. `2607:6bc0::/48`, DNS (`udp`/`tcp` 53) nur für Sockets von uid 0 und bei IPv6 ICMPv6-Nachbarerkennung erlauben. Die Tabelle `nat` MUST unberührt bleiben. Schlägt eine Regel fehl oder ist weder `iptables` noch `iptables-legacy` nutzbar, MUST der Entrypoint mit einem Fehler enden, ohne die Konfiguration zu rendern oder CLIProxyAPI zu starten. Danach MUST er mit `setpriv` auf den Benutzer `cpa` ohne Bounding- und Inheritable-Capabilities und mit `no_new_privs` wechseln. Compose MUST nur `cliproxyapi` und `cpa-login` die Capability `NET_ADMIN` geben; tollgate MUST keine zusätzlichen Capabilities erhalten und erbt die Regeln über den gemeinsamen Namespace.

#### Scenario: Regeln vor dem Start
- **WHEN** der Entrypoint mit nutzbarem `iptables` läuft
- **THEN** ist der erste Regelaufruf je Familie `-P OUTPUT DROP`, es folgen genau die erlaubten Regeln, und erst danach startet CLIProxyAPI als `cpa` ohne Capabilities.

#### Scenario: Fail-closed
- **WHEN** eine `iptables`- oder `ip6tables`-Regel fehlschlägt oder kein Backend nutzbar ist
- **THEN** endet der Entrypoint mit Exit-Code ungleich 0, und weder `render-config.sh` noch CLIProxyAPI werden aufgerufen.

#### Scenario: Legacy-Backend
- **WHEN** `iptables` (nft) `-S OUTPUT` nicht ausführen kann, `iptables-legacy` aber schon
- **THEN** setzt der Entrypoint alle IPv4-Regeln mit `iptables-legacy`.

#### Scenario: Egress im laufenden Stack
- **WHEN** `just smoke` läuft
- **THEN** baut der Namespace eine Verbindung zu `api.anthropic.com:443` auf, eine Verbindung zu einem fremden Host scheitert, und das Image ohne `NET_ADMIN` startet nicht.

### Requirement: Kein entfernter Modellkatalog

Der Entrypoint MUST CLIProxyAPI immer mit `-local-model` starten, auch beim Login. Der Login-Dienst `cpa-login` MUST denselben Entrypoint nutzen und nur `-claude-login -no-browser` als Argumente übergeben.

#### Scenario: Server-Start
- **WHEN** der Entrypoint ohne Argumente läuft
- **THEN** startet CLIProxyAPI mit `-config /run/cpa/config.yaml -local-model`.

#### Scenario: Login
- **WHEN** `cpa-login` startet
- **THEN** startet CLIProxyAPI mit `-local-model -claude-login -no-browser` hinter denselben Egress-Regeln.
