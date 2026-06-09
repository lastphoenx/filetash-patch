# pCloud Backend für Filestash

pCloud OAuth 2.0 Integration für Filestash File Manager.

## Features

- ✅ OAuth 2.0 Authorization Code Flow
- ✅ Vollständige Dateiverwaltung (CRUD)
- ✅ Multi-Account Switching (gleichzeitiger Login mehrerer Accounts)
- ✅ Bearer Token Query-Parameter Authentifizierung
- ✅ Korrekte Path-Encoding für pCloud API

## Quick Start

### 1. pCloud OAuth App erstellen

1. Developer Account auf [pcloud.com](https://www.pcloud.com) erstellen
2. In Developer Console neue OAuth App registrieren
3. **Wichtig:** App-Folder Restriction DEAKTIVIEREN
4. Mindestens `/Backup` Ordner-Zugriff konfigurieren
5. Client ID und Secret erhalten

### 2. Konfiguration

Kopiere `docker-compose-pcloud.yml` und ersetze die Platzhalter:

```yaml
PCLOUD_CLIENT_ID=<YOUR_CLIENT_ID>
PCLOUD_CLIENT_SECRET=<YOUR_CLIENT_SECRET>
APPLICATION_URL=your-domain.com
```

### 3. Docker Container starten

```bash
docker-compose -f docker-compose-pcloud.yml up -d
```

## Dateien

- **backend-pcloud-index.go** — pCloud Backend Plugin (v1.0.2)
- **docker-compose-pcloud.yml** — Docker-Komposition mit pCloud-Konfiguration

## Technische Details

### Bearer Token Authentication

pCloud akzeptiert Access Token als Query-Parameter, nicht als Authorization Header:

```
https://api.pcloud.com/listfolder?access_token=TOKEN&folderid=0
```

### Path Encoding

Das Plugin behandelt Pfade speziell, um Slashes korrekt zu handhaben:

```go
// URL-kodierte Slashes werden konvertiert: %2F → /
strings.ReplaceAll(url.QueryEscape(pathVal), "%2F", "/")
```

Dies ist erforderlich, da pCloud die Slashes als Pfad-Trennzeichen interpretiert.

### OAuth Ablauf

1. Benutzer klickt pCloud-Button
2. Wird zu `https://my.pcloud.com/oauth2/authorize` weitergeleitet
3. Genehmigt Zugriff
4. Wird zurück weitergeleitet mit `code` Parameter
5. Plugin tauscht Code gegen Access Token
6. Token wird in Browser-Cookies gespeichert

## Implementierte Funktionen

- `Ls(path)` — Dateien/Ordner auflisten
- `Cat(path)` — Dateiinhalt lesen
- `Mkdir(path)` — Ordner erstellen
- `Rm(path)` — Datei/Ordner löschen
- `Mv(from, to)` — Verschieben/Umbenennen
- `Touch(path)` — Leere Datei erstellen
- `Save(path, data)` — Datei hochladen

## Bekannte Einschränkungen

- Dateien sind nur sichtbar, wenn pCloud OAuth-App mit `/Backup` Zugriff konfiguriert ist
- Token wird in Browser-Cookies gespeichert (nicht persistent über Browser-Restart)

## Vollständige Dokumentation

Siehe `/doku` private Repository für erweiterte Dokumentation:
- Detaillierte OAuth-Flow Erklärung
- Versions-Historie (1.0.0 → 1.0.2)
- Alle Bugfixes und Testings
- Multi-Account Switching Feature

## Version

**v1.0.2** — Path-Encoding Fix für %2F → / Konvertierung

## License

Siehe LICENSE Datei.
