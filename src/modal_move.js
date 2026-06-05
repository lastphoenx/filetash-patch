import { createElement } from "../../lib/skeleton/index.js";
import rxjs, { effect } from "../../lib/rx.js";
import { qs } from "../../lib/dom.js";
import { MODAL_RIGHT_BUTTON } from "../../components/modal.js";
import { basename } from "../../lib/path.js";
import { ls } from "./model_files.js";
import t from "../../locales/index.js";

export default function(render, fromPath) {
    const getDirname = (p) => p.replace(/\/[^\/]*\/?$/, "") || "/";
    const isDirectory = fromPath.endsWith("/");
    const filename = basename(fromPath.replace(/\/$/, ""));
    const startPath = getDirname(fromPath.replace(/\/$/, ""));

    let currentPath = startPath;
    let selectedPath = null;

    const $modal = createElement(`
        <div style="width:min(560px,88vw);">
            <div style="
                display:flex;align-items:center;gap:10px;
                padding:10px 14px;
                background:#f0f4f8;
                border-radius:6px;
                margin-bottom:10px;
            ">
                <span style="font-size:1.3em;flex-shrink:0">📄</span>
                <div style="overflow:hidden">
                    <div style="font-size:0.78em;color:#888;margin-bottom:1px">Verschieben</div>
                    <div style="font-weight:600;color:#222;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">${filename}</div>
                </div>
            </div>
            <div class="move-breadcrumb" style="
                display:flex;align-items:center;flex-wrap:wrap;gap:4px;
                font-size:0.82em;
                padding:6px 4px;
                min-height:28px;
            "></div>
            <div class="move-listing" style="
                border:1px solid #ddd;
                border-radius:6px;
                min-height:180px;
                max-height:min(300px,40vh);
                overflow-y:auto;
                background:#fff;
            "></div>
            <div style="margin-top:8px;font-size:0.8em;color:#999">
                Ordner anklicken = Ziel auswählen &nbsp;·&nbsp; ▶ = Ordner öffnen
            </div>
            <div class="modal-error-message" style="color:#e53935;font-size:0.83em;min-height:16px;margin-top:4px;"></div>
        </div>
    `);

    const ret = new rxjs.Subject();
    const $listing = qs($modal, ".move-listing");
    const $breadcrumb = qs($modal, ".move-breadcrumb");
    const $error = qs($modal, ".modal-error-message");

    function updateBreadcrumb(path) {
        $breadcrumb.innerHTML = "";
        const parts = path.replace(/^\//, "").replace(/\/$/, "").split("/");
        const paths = ["/"];
        parts.filter(Boolean).forEach((p, i) => {
            paths.push("/" + parts.slice(0, i + 1).join("/") + "/");
        });
        const labels = ["/", ...parts.filter(Boolean)];
        labels.forEach((label, i) => {
            const $chip = createElement(`
                <span data-nav="${paths[i]}" style="
                    padding:2px 8px;
                    border-radius:12px;
                    background:${i === labels.length - 1 ? "#1976d2" : "#e8eef3"};
                    color:${i === labels.length - 1 ? "#fff" : "#444"};
                    cursor:pointer;
                    white-space:nowrap;
                ">${label}</span>
            `);
            $breadcrumb.appendChild($chip);
            if (i < labels.length - 1) {
                $breadcrumb.appendChild(createElement(`<span style="color:#bbb">›</span>`));
            }
        });

        effect(rxjs.fromEvent($breadcrumb, "click").pipe(
            rxjs.tap((e) => {
                const $chip = e.target.closest("[data-nav]");
                if ($chip) loadPath($chip.getAttribute("data-nav"));
            }),
        ));
    }

    function loadPath(path) {
        currentPath = path;
        selectedPath = null;
        updateBreadcrumb(path);
        $listing.innerHTML = `<div style="padding:16px;color:#aaa;text-align:center">⏳ Lade...</div>`;

        ls(path).pipe(rxjs.first()).subscribe({
            next: ({ files }) => {
                const dirs = (files || []).filter(f => f.type === "directory");
                $listing.innerHTML = "";

                if (path !== "/") {
                    const parentPath = getDirname(path.replace(/\/$/, "")) || "/";
                    const $back = createElement(`
                        <div data-nav="${parentPath}" style="
                            padding:10px 16px;cursor:pointer;
                            border-bottom:1px solid #f0f0f0;
                            display:flex;align-items:center;gap:10px;
                            color:#666;font-size:0.9em;
                        ">⬆&nbsp;&nbsp;..</div>
                    `);
                    $listing.appendChild($back);
                }

                if (dirs.length === 0) {
                    $listing.appendChild(createElement(`
                        <div style="padding:16px;color:#bbb;text-align:center;font-size:0.9em">Keine Unterordner</div>
                    `));
                }

                dirs.forEach(f => {
                    const dirPath = path.replace(/\/?$/, "/") + f.name + "/";
                    const $row = createElement(`
                        <div data-path="${dirPath}" style="
                            padding:10px 16px;cursor:pointer;
                            border-bottom:1px solid #f0f0f0;
                            display:flex;align-items:center;gap:10px;
                        ">
                            <span style="font-size:1.1em;flex-shrink:0">📁</span>
                            <span style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:0.92em">${f.name}</span>
                            <span data-nav="${dirPath}" style="
                                padding:4px 12px;border-radius:4px;
                                background:#e8eef3;font-size:0.82em;color:#444;
                                flex-shrink:0;font-weight:600;
                            ">▶</span>
                        </div>
                    `);
                    $listing.appendChild($row);
                });

                effect(rxjs.fromEvent($listing, "click").pipe(
                    rxjs.tap((e) => {
                        const $nav = e.target.closest("[data-nav]");
                        if ($nav) {
                            e.stopPropagation();
                            loadPath($nav.getAttribute("data-nav"));
                            return;
                        }
                        const $row = e.target.closest("[data-path]");
                        if ($row) {
                            $listing.querySelectorAll("[data-path]").forEach(r => {
                                r.style.background = "";
                                r.style.fontWeight = "";
                            });
                            selectedPath = $row.getAttribute("data-path");
                            $row.style.background = "#e3f2fd";
                            $row.style.fontWeight = "600";
                            $error.textContent = "";
                        }
                    }),
                ));
            },
            error: () => {
                $listing.innerHTML = `<div style="padding:16px;color:#e53935;text-align:center">Fehler beim Laden</div>`;
            }
        });
    }

    render($modal, function(id) {
        if (id !== MODAL_RIGHT_BUTTON) return;
        const targetDir = (selectedPath || currentPath).replace(/\/?$/, "/");
        const newPath = targetDir + filename + (isDirectory ? "/" : "");
        if (newPath === fromPath) {
            $error.textContent = "Ziel ist identisch mit Quelle";
            return ret.toPromise();
        }
        ret.next(newPath);
        ret.complete();
    });

    loadPath(startPath);
    return ret.toPromise();
}
