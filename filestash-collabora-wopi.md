# Filestash – Collabora Online (WOPI) Integration

## Übersicht

Collabora Online ermöglicht das direkte Bearbeiten von Office-Dokumenten (`.docx`, `.xlsx`, `.pptx`, `.odt` etc.) im Browser – direkt aus Filestash heraus, ohne Download.

---

## Infrastruktur

| Komponente | Host | IP/URL | Port |
|------------|------|--------|------|
| Filestash | CT 122 | 192.168.131.32 | 8334 |
| Collabora (coolwsd) | VM 102 auf pve2 (nativ) | 172.19.0.2 | 9980 |
| nginx Reverse Proxy | nginx CT | — | 443 |
| Collabora extern | — | https://o365.santinel.li | 443 |

Der `filestash`-Docker-Container läuft auf CT 122. Collabora läuft als **nativer systemd-Dienst** (`coolwsd`) auf VM 102 (`o365.santinel.li`, Debian 13 Trixie) – nicht mehr als Docker-Container. Die interne Erreichbarkeit von CT 122 zu VM 102 erfolgt über `172.19.0.2:9980`.

---

## Filestash Admin-Einstellungen

Unter `http://192.168.131.32:8334/admin` → **OFFICE**:

| Feld | Wert | Erklärung |
|------|------|-----------|
| Enable | ✅ | WOPI aktivieren |
| Filestash Server | `https://files.santinel.li` | URL von Filestash aus Sicht des Collabora-Containers |
| Office Server | `http://172.19.0.2:9980` | Collabora intern (Docker-Netzwerk IP) |
| Rewrite Discovery | `https://o365.santinel.li` | Externe Collabora-URL für den Browser |

**Wichtig:** `Office Server` muss die **Docker-interne IP** sein, nicht `localhost` – `localhost` würde im Filestash-Container auf sich selbst zeigen, nicht auf den Collabora-Container.

**Rewrite Discovery** ist nötig weil Collabora in der Discovery-Antwort `http://172.19.0.2:9980` zurückgibt – das ist für den Browser nicht erreichbar. Filestash ersetzt diese URL durch `https://o365.santinel.li`.

---

## Collabora Konfiguration (coolwsd.xml)

Datei: `/etc/coolwsd/coolwsd.xml` auf dem Collabora-Host (`root@o365.santinel.li`, VM 102 auf pve2)

In der `<alias_groups>` Sektion müssen alle Hosts eingetragen sein die Collabora aufrufen dürfen:

```xml
<alias_groups desc="default mode is 'first'...">
    <group>
        <host allow="true" desc="Nextcloud">https://cloud.santinel.li:443</host>
    </group>
    <group>
        <host allow="true" desc="Office">https://office.santinel.li:443</host>
    </group>
    <group>
        <host allow="true" desc="Office2">https://office2.santinel.li:443</host>
    </group>
    <group>
        <host allow="true" desc="Filestash extern">https://files.santinel.li</host>
    </group>
    <group>
        <host allow="true" desc="Filestash intern">https://192.168.131.32:8334</host>
    </group>
</alias_groups>
```

Nach Änderung:
```bash
sudo systemctl restart coolwsd
sudo systemctl status coolwsd
```

---

## nginx Konfiguration

Datei: `/etc/nginx/sites-available/files.santinel.li.conf`

Der WOPI-Endpoint `/api/wopi/` muss von der Authentik-Authentifizierung **ausgenommen** werden – Collabora authentifiziert sich selbst über WOPI-Tokens:

```nginx
server {
    listen 443 ssl http2;
    server_name files.santinel.li;

    # SSL, Logging, Headers etc. ...

    include /etc/nginx/snippets/authentik-outpost.conf;

    # WOPI-Endpunkt fuer Collabora - kein Authentik
    # Collabora authentifiziert selbst ueber WOPI-Access-Tokens
    location /api/wopi/ {
        include /etc/nginx/snippets/proxy-headers.conf;
        proxy_pass http://192.168.131.32:8334;
    }

    # Alles andere mit Authentik geschuetzt
    location / {
        include /etc/nginx/snippets/authentik-auth.conf;
        include /etc/nginx/snippets/proxy-headers.conf;
        proxy_pass http://192.168.131.32:8334;
    }
}
```

Nach Änderung:
```bash
nginx -t && systemctl reload nginx
```

---

## Wie es funktioniert (Ablauf)

```
Browser klickt .docx
        ↓
Filestash holt Discovery von http://172.19.0.2:9980/hosting/discovery
        ↓
Filestash rewritet URLs: 172.19.0.2:9980 → https://o365.santinel.li
        ↓
Browser lädt Collabora-Editor von https://o365.santinel.li
        ↓
Collabora holt Datei via WOPI: GET https://files.santinel.li/api/wopi/files/...
        ↓
nginx leitet /api/wopi/ ohne Authentik weiter → Filestash → SMB → pi-nas
        ↓
Collabora zeigt Dokument im Browser-Editor
        ↓
Beim Speichern: Collabora POST https://files.santinel.li/api/wopi/files/.../contents
        ↓
Filestash schreibt via SMB auf pi-nas
```

---

## Unterstützte Formate

Collabora CODE unterstützt direkt in Filestash:

| Format | Aktion |
|--------|--------|
| .docx, .doc, .odt | Bearbeiten |
| .xlsx, .xls, .ods | Bearbeiten |
| .pptx, .ppt, .odp | Bearbeiten |
| .pdf | Anzeigen |
| .txt, .csv | Bearbeiten |

---

## Troubleshooting

**Datei öffnet sich nicht / nur Download-Button:**
- Prüfen ob WOPI in Filestash Admin aktiviert ist
- `curl http://172.19.0.2:9980/hosting/discovery` vom Filestash-Container aus testen:
  `docker exec filestash curl http://172.19.0.2:9980/hosting/discovery | head -5`

**Collabora lädt aber hängt bei "Initialisierung...":**
- Wahrscheinlich blockiert Authentik den `/api/wopi/`-Endpoint
- nginx-Config prüfen: `location /api/wopi/` muss ohne `authentik-auth.conf` konfiguriert sein

**Collabora gibt Fehler beim Laden der Datei:**
- `files.santinel.li` muss in `coolwsd.xml` unter `alias_groups` eingetragen sein
- `sudo systemctl restart coolwsd` nach Änderung

**Docker-Container IPs geändert:**
- IPs prüfen: `docker inspect filestash_wopi | grep IPAddress`
- Office Server in Filestash Admin aktualisieren

---

## Wiederherstellung nach Neuinstallation CT 122

Falls der Filestash-Container neu erstellt wird, muss geprüft werden ob Collabora wieder die IP `172.19.0.2` bekommt:

```bash
docker inspect filestash_wopi | grep IPAddress
```

Falls andere IP → Filestash Admin → Office Server URL anpassen.

Die WOPI-Config wird in `/app/data/state/config/config.json` gespeichert – dieses Volume sollte persistiert sein.

---

## Optional: PDF-Dateien über Collabora öffnen

Standardmässig öffnet Filestash PDF-Dateien mit dem eingebauten pdfjs-Viewer.
Falls dieser nicht funktioniert (z.B. schwarzes Bild), kann Collabora als PDF-Viewer konfiguriert werden.

### Umstellen auf Collabora

Auf CT 130 (Build):

```bash
ssh root@192.168.131.30
sed -i 's/return \["pdf", { mime }\];/return ["office", { mime }];/' \
    /root/filestash/public/assets/pages/viewerpage/mimetype.js

# Prüfen:
grep -n "pdf\|office" /root/filestash/public/assets/pages/viewerpage/mimetype.js | head -5

# Bauen und deployen:
cd /root/filestash && make build 2>&1 && \
scp dist/filestash root@192.168.131.32:/tmp/filestash-new && \
ssh root@192.168.131.32 "docker cp /tmp/filestash-new filestash:/app/filestash && docker restart filestash && echo Done"
```

### Zurück auf eingebauten PDF-Viewer

```bash
sed -i 's/return \["office", { mime }\];/return ["pdf", { mime }];/' \
    /root/filestash/public/assets/pages/viewerpage/mimetype.js
# Dann bauen und deployen wie oben
```

### Hinweis

- Eingebauter pdfjs-Viewer: sofort, kein Server-Roundtrip
- Collabora PDF-Viewer: ~10 Sekunden Ladezeit, dafür zuverlässiger
- Bei Problemen mit pdfjs zuerst Browser-Extensions deaktivieren (Inkognito-Modus testen)
