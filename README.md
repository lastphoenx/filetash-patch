# Filestash Move/Verschieben Patch

**Move-Feature + Keyboard Shortcuts für Filestash** – Community-Patch basierend auf Issue [#801](https://github.com/mickael-kerjean/filestash/issues/801).

Läuft produktiv seit April 2026. **Benutzung auf eigenes Risiko.**

> ⚠️ **Hinweis:** Dieser Code wurde 100% KI-generiert und manuell getestet. Benutzung auf eigenes Risiko.

---

## Features

- **VERSCHIEBEN-Button** – für Einzel- und Mehrfach-Selektion
- **Ordner-Picker Modal** – mit Breadcrumb-Navigation, responsiv
- **Tastaturkürzel** – F2 (Rename), Del (Delete), M (Move)
- **Statusmeldung** – unterscheidet "verschoben" vs. "umbenannt"
- **Delete-Bestätigung** – Eingabefeld erwartet `remove` statt Dateiname
- **Modal-Breite** – erhöht auf 600px

---

## Installation

### Voraussetzungen
- Filestash Source (Commit `272eb29f`)
- Go 1.26+
- Make

### Setup

```bash
# 1. Filestash klonen
git clone https://github.com/mickael-kerjean/filestash.git
cd filestash && git checkout 272eb29f

# 2. Patch-Dateien kopieren
cp src/*.js public/assets/pages/filespage/
cp src/modal.css public/assets/components/
cp src/index.frontoffice.html public/

# 3. Bauen
make build 2>&1

# 4. Deployen (Docker beispiel)
scp dist/filestash root@<HOST>:/tmp/filestash-new
ssh root@<HOST> "docker cp /tmp/filestash-new filestash:/app/filestash && docker restart filestash"
```

---

## Verwendung

**Datei auswählen** → **VERSCHIEBEN-Button** klickt → **Zielordner wählen** → **OK**

**Shortcuts:** F2 (Rename), Del (Delete), M (Move) – funktioniert wenn Dateien selektiert sind und keine Modal offen.

---

## Patch-Struktur

```
src/
├── modal_move.js           – Ordner-Picker Modal
├── ctrl_submenu.js         – VERSCHIEBEN-Button + Handler
├── model_files.js          – Statusmeldung
├── modal.css               – Modal-Breite
└── index.frontoffice.html  – Shortcuts
```

---

## Upstream-Status

Issue [#801](https://github.com/mickael-kerjean/filestash/issues/801) seit 2022 offen. Dieser Patch ist eine **Community-Lösung**, nicht offiziell supported.

---

## Verwandte Dokumentation

- [filestash-collabora-wopi.md](filestash-collabora-wopi.md) – Collabora Online (WOPI) Integration: Infrastruktur, Filestash-Admin-Einstellungen, nginx-Config, Troubleshooting und PDF-Viewer-Option

---

## License

MIT – wie Filestash
