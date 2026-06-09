# pCloud Backend für Filestash

pCloud OAuth 2.0 Integration für Filestash File Manager.

**Aktuelle Version:** 1.1.4

## Features

- OAuth 2.0 Authorization Code Flow (EU/US API-Host)
- CRUD: Listen, Upload, Move, Rename, Delete (Papierkorb)
- Ordner-ZIP-Download über Filestash `api/files/zip`
- `fileid`-basierte Deletes (Papierkorb, eindeutig bei gleichem Datei-/Ordnernamen)
- Path-Encoding für pCloud REST (`path` / `topath` ohne `%2F`-Slash-Bug)
- Optional: `PCLOUD_ROOT_FOLDERID` als Startansicht (kein Pfad-Chroot)

## Abgrenzung zu Samba / anderen Backends

Dieses Plugin ist **`plg_backend_pcloud`** — isoliert vom Filestash-Samba-Plugin (`plg_backend_samba`) und allen anderen Backends. Es werden nur Dateien in diesem Ordner geändert:

- `backend-pcloud-index.go` → Build nach `server/plugin/plg_backend_pcloud/index.go`
- `docker-compose-pcloud.yml` → Beispiel-Env (ohne `PCLOUD_ROOT_PATH`-Chroot)

`model_files.js` (Move-Patch) ist **Frontend** und verbessert Statusmeldungen für alle Backends (Ordner vs. Datei), ohne Samba-Backend-Code anzufassen.

## Quick Start

### 1. pCloud OAuth App

1. Developer Account auf [pcloud.com](https://www.pcloud.com)
2. OAuth App registrieren
3. **App-Folder Restriction DEAKTIVIEREN**
4. Redirect URI: `https://<APPLICATION_URL>/login`
5. Client ID und Secret notieren

### 2. Build einbinden

```bash
cp backend-pcloud-index.go /root/filestash/server/plugin/plg_backend_pcloud/index.go
# Optional: model_files.js aus Move-Patch (Ordner/Datei-Meldungen)
cp model_files.js /root/filestash/public/assets/pages/filespage/
cd /root/filestash && make build
```

### 3. Deploy

```bash
scp dist/filestash root@<PROD>:/tmp/filestash-new
ssh root@<PROD> "docker cp /tmp/filestash-new filestash:/app/filestash && \
  docker commit filestash filestash-custom:latest && docker restart filestash"
```

## Dateien in diesem Patch

| Datei | Zweck |
|-------|--------|
| `backend-pcloud-index.go` | pCloud Backend (v1.1.4) |
| `docker-compose-pcloud.yml` | Compose-Beispiel mit OAuth-Env |
| `model_files.js` | UI-Meldungen Ordner vs. Datei (optional, Frontend) |

## Versionshistorie

| Version | Inhalt |
|---------|--------|
| **1.1.4** | ZIP-Download: `isfolder` robust (0/1/bool/icon), `FTime` Unix-Sekunden, `Cat` via `fileid` |
| **1.1.3** | Delete via `fileid`; `resolveEntry`; UI-Meldungen Ordner/Datei |
| **1.1.2** | Kein `PCLOUD_ROOT_PATH`-Chroot — volle pCloud-Root sichtbar |
| **1.1.1** | Papierkorb: `deletefile`/`deletefolder` + `trashFolder`, kein `deletefolderrecursive` |
| **1.1.0** | Stat, ensurePath, multipart Upload, sicheres Move/Rm |
| **1.0.2** | Path-Encoding `%2F` → `/` |
| **1.0.0** | Initial OAuth |

## Technische Referenz

Design orientiert an `pcloud_bin_lib.py` (Pfad-Normalisierung, Move mit vollem `topath`, multipart Upload). Vollständige Doku, Pipeline und Rollback: privates Repo `doku` → `pve2/vm/122-filestash/README.md`.

## License

Siehe LICENSE im Repository-Root.
