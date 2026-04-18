"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// ── Storage keys ──────────────────────────────────────────────────────────────
const NOTES_KEY = "raig.notepad.data";
const SCREENSHOTS_KEY = "raig.notepad.screenshots";

// ── Types ─────────────────────────────────────────────────────────────────────
type EntryType = "text" | "secret";

type NoteEntry = {
  id: string;
  key: string;
  value: string;
  type: EntryType;
  createdAt: number;
};

type NoteSection = {
  id: string;
  title: string;
  color: string;
  entries: NoteEntry[];
};

type NotePadData = {
  sections: NoteSection[];
};

type Screenshot = {
  id: string;
  dataUrl: string;       // base64 data URL
  label: string;
  createdAt: number;
  sizeKB: number;
};

// ── Constants ─────────────────────────────────────────────────────────────────
const SECTION_COLORS = [
  "#1a73e8", "#1e8e3e", "#e37400",
  "#d93025", "#8430ce", "#00838f", "#c2185b",
];

const DEFAULT_DATA: NotePadData = {
  sections: [
    {
      id: "api-keys",
      title: "API Keys",
      color: "#1a73e8",
      entries: [
        { id: "demo-1", key: "Delta Exchange API Key", value: "", type: "secret", createdAt: Date.now() },
        { id: "demo-2", key: "Angel One Client ID",    value: "", type: "secret", createdAt: Date.now() },
      ],
    },
    { id: "notes", title: "Notes", color: "#1e8e3e", entries: [] },
  ],
};

// ── Helpers ───────────────────────────────────────────────────────────────────
function uid() {
  return Math.random().toString(36).slice(2, 10);
}

function loadNotes(): NotePadData {
  if (typeof window === "undefined") return DEFAULT_DATA;
  try {
    const raw = window.localStorage.getItem(NOTES_KEY);
    return raw ? (JSON.parse(raw) as NotePadData) : DEFAULT_DATA;
  } catch { return DEFAULT_DATA; }
}

function saveNotes(data: NotePadData) {
  try { window.localStorage.setItem(NOTES_KEY, JSON.stringify(data)); } catch { /* quota */ }
}

function loadScreenshots(): Screenshot[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(SCREENSHOTS_KEY);
    return raw ? (JSON.parse(raw) as Screenshot[]) : [];
  } catch { return []; }
}

function saveScreenshots(list: Screenshot[]) {
  try { window.localStorage.setItem(SCREENSHOTS_KEY, JSON.stringify(list)); } catch { /* quota */ }
}

// ── Shared style helpers ───────────────────────────────────────────────────────
const iconBtnStyle: React.CSSProperties = {
  background: "none", border: "none", cursor: "pointer",
  padding: "3px 5px", borderRadius: 4, fontSize: 13,
  lineHeight: 1, color: "var(--text-muted)",
  display: "inline-flex", alignItems: "center",
};

const colHeaderStyle: React.CSSProperties = {
  fontSize: 10, fontWeight: 600, letterSpacing: "0.1em",
  textTransform: "uppercase", color: "var(--text-muted)",
};

const inputStyle: React.CSSProperties = {
  padding: "5px 10px", borderRadius: "var(--radius-input)",
  border: "1px solid var(--border)", background: "var(--surface)",
  fontSize: 13, color: "var(--text-primary)", outline: "none", width: "100%",
};

// ── SecretValue ───────────────────────────────────────────────────────────────
function SecretValue({ value }: { value: string }) {
  const [visible, setVisible] = useState(false);
  const isEmpty = !value.trim();
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
      <span style={{
        fontFamily: "var(--font-mono)", fontSize: 12,
        color: isEmpty ? "var(--text-muted)" : "var(--text-primary)",
        letterSpacing: visible ? 0 : 2,
        userSelect: visible ? "text" : "none",
        wordBreak: "break-all",
      }}>
        {isEmpty ? "—" : visible ? value : "•".repeat(Math.min(value.length, 24))}
      </span>
      {!isEmpty && (
        <button type="button" onClick={() => setVisible(v => !v)} title={visible ? "Hide" : "Show"}
          style={{ background: "none", border: "none", cursor: "pointer", padding: "2px 4px",
            borderRadius: 4, color: "var(--text-muted)", fontSize: 11, lineHeight: 1 }}>
          {visible ? "hide" : "show"}
        </button>
      )}
    </span>
  );
}

// ── EditRow ───────────────────────────────────────────────────────────────────
function EditRow({ entry, onSave, onCancel }: {
  entry: NoteEntry;
  onSave: (u: NoteEntry) => void;
  onCancel: () => void;
}) {
  const [key, setKey] = useState(entry.key);
  const [value, setValue] = useState(entry.value);
  const [type, setType] = useState<EntryType>(entry.type);
  const keyRef = useRef<HTMLInputElement>(null);
  useEffect(() => { keyRef.current?.focus(); }, []);

  return (
    <form onSubmit={e => { e.preventDefault(); if (key.trim()) onSave({ ...entry, key: key.trim(), value: value.trim(), type }); }}
      style={{ display: "contents" }}>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1.6fr auto auto auto", gap: 8,
        alignItems: "center", padding: "6px 0", borderBottom: "1px solid var(--border-subtle)" }}>
        <input ref={keyRef} value={key} onChange={e => setKey(e.target.value)}
          placeholder="Label / Key name" style={inputStyle} />
        <input value={value} onChange={e => setValue(e.target.value)} placeholder="Value"
          type={type === "secret" ? "password" : "text"} autoComplete="off"
          style={{ ...inputStyle, fontFamily: type === "secret" ? "var(--font-mono)" : "inherit" }} />
        <select value={type} onChange={e => setType(e.target.value as EntryType)}
          style={{ ...inputStyle, width: "auto", cursor: "pointer" }}>
          <option value="text">Text</option>
          <option value="secret">Secret</option>
        </select>
        <button type="submit" style={{ padding: "5px 14px", borderRadius: "var(--radius-input)",
          border: "none", background: "var(--accent)", color: "#fff",
          fontSize: 12, fontWeight: 600, cursor: "pointer", whiteSpace: "nowrap" }}>Save</button>
        <button type="button" onClick={onCancel} style={{ padding: "5px 12px",
          borderRadius: "var(--radius-input)", border: "1px solid var(--border)",
          background: "var(--surface)", color: "var(--text-secondary)",
          fontSize: 12, cursor: "pointer" }}>Cancel</button>
      </div>
    </form>
  );
}

// ── Section ───────────────────────────────────────────────────────────────────
function Section({ section, onUpdate, onDelete }: {
  section: NoteSection;
  onUpdate: (u: NoteSection) => void;
  onDelete: () => void;
}) {
  const [editingId, setEditingId]   = useState<string | null>(null);
  const [addingNew, setAddingNew]   = useState(false);
  const [editTitle, setEditTitle]   = useState(false);
  const [titleDraft, setTitleDraft] = useState(section.title);
  const [collapsed, setCollapsed]   = useState(false);

  function saveTitle() {
    if (titleDraft.trim()) onUpdate({ ...section, title: titleDraft.trim() });
    setEditTitle(false);
  }

  return (
    <div style={{ background: "var(--surface)", borderRadius: "var(--radius-card)",
      border: "1px solid var(--border)", overflow: "hidden", boxShadow: "var(--shadow-xs)" }}>

      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", gap: 10, padding: "10px 16px",
        borderBottom: collapsed ? "none" : "1px solid var(--border-subtle)",
        background: "var(--surface-2)", cursor: "pointer" }}>
        <span style={{ width: 10, height: 10, borderRadius: "50%",
          background: section.color, flexShrink: 0 }} />

        {editTitle ? (
          <input autoFocus value={titleDraft} onChange={e => setTitleDraft(e.target.value)}
            onBlur={saveTitle}
            onKeyDown={e => { if (e.key === "Enter") saveTitle(); if (e.key === "Escape") setEditTitle(false); }}
            onClick={e => e.stopPropagation()}
            style={{ fontSize: 13, fontWeight: 600, color: "var(--text-primary)",
              background: "var(--surface)", border: "1px solid var(--accent)",
              borderRadius: 4, padding: "2px 8px", outline: "none", flex: 1 }} />
        ) : (
          <span style={{ fontSize: 13, fontWeight: 600, color: "var(--text-primary)", flex: 1 }}
            onClick={() => setCollapsed(c => !c)}>
            {section.title}
            <span style={{ marginLeft: 8, fontSize: 11, fontWeight: 400, color: "var(--text-muted)" }}>
              ({section.entries.length})
            </span>
          </span>
        )}

        <div style={{ display: "flex", gap: 4, marginLeft: "auto" }}>
          <button type="button" onClick={e => { e.stopPropagation(); setTitleDraft(section.title); setEditTitle(true); }}
            title="Rename" style={iconBtnStyle}>✏️</button>
          <button type="button" title="Delete section"
            style={{ ...iconBtnStyle, color: "var(--red)" }}
            onClick={e => { e.stopPropagation(); if (window.confirm(`Delete section "${section.title}"?`)) onDelete(); }}>
            🗑️
          </button>
          <button type="button" onClick={e => { e.stopPropagation(); setCollapsed(c => !c); }}
            title={collapsed ? "Expand" : "Collapse"} style={iconBtnStyle}>
            {collapsed ? "▶" : "▼"}
          </button>
        </div>
      </div>

      {!collapsed && (
        <div style={{ padding: "8px 16px 12px" }}>
          {section.entries.length > 0 && (
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1.6fr auto", gap: 8,
              padding: "4px 0 6px", borderBottom: "1px solid var(--border-subtle)", marginBottom: 4 }}>
              <span style={colHeaderStyle}>Label / Key</span>
              <span style={colHeaderStyle}>Value</span>
              <span style={colHeaderStyle} />
            </div>
          )}

          {section.entries.map(entry =>
            editingId === entry.id ? (
              <EditRow key={entry.id} entry={entry}
                onSave={u => { onUpdate({ ...section, entries: section.entries.map(e => e.id === u.id ? u : e) }); setEditingId(null); }}
                onCancel={() => setEditingId(null)} />
            ) : (
              <div key={entry.id} style={{ display: "grid", gridTemplateColumns: "1fr 1.6fr auto",
                gap: 8, alignItems: "center", padding: "7px 0",
                borderBottom: "1px solid var(--border-subtle)" }}>
                <span style={{ fontSize: 13, fontWeight: 500, color: "var(--text-secondary)",
                  overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {entry.key}
                </span>
                <span style={{ overflow: "hidden" }}>
                  {entry.type === "secret"
                    ? <SecretValue value={entry.value} />
                    : <span style={{ fontSize: 13, color: entry.value ? "var(--text-primary)" : "var(--text-muted)", wordBreak: "break-all" }}>
                        {entry.value || "—"}
                      </span>}
                </span>
                <div style={{ display: "flex", gap: 4 }}>
                  <button type="button" title="Copy" style={iconBtnStyle} disabled={!entry.value}
                    onClick={() => navigator.clipboard.writeText(entry.value).catch(() => {})}>📋</button>
                  <button type="button" title="Edit" style={iconBtnStyle}
                    onClick={() => setEditingId(entry.id)}>✏️</button>
                  <button type="button" title="Delete" style={{ ...iconBtnStyle, color: "var(--red)" }}
                    onClick={() => { if (window.confirm(`Delete "${entry.key}"?`)) onUpdate({ ...section, entries: section.entries.filter(e => e.id !== entry.id) }); }}>
                    ✕
                  </button>
                </div>
              </div>
            )
          )}

          {addingNew ? (
            <EditRow entry={{ id: uid(), key: "", value: "", type: "text", createdAt: Date.now() }}
              onSave={entry => { onUpdate({ ...section, entries: [...section.entries, entry] }); setAddingNew(false); }}
              onCancel={() => setAddingNew(false)} />
          ) : (
            <button type="button" onClick={() => setAddingNew(true)}
              style={{ marginTop: 8, display: "inline-flex", alignItems: "center", gap: 6,
                padding: "5px 12px", borderRadius: "var(--radius-chip)",
                border: "1px dashed var(--border-strong)", background: "transparent",
                color: "var(--accent)", fontSize: 12, fontWeight: 500, cursor: "pointer" }}>
              + Add entry
            </button>
          )}
        </div>
      )}
    </div>
  );
}

// ── AddSectionRow ─────────────────────────────────────────────────────────────
function AddSectionRow({ onAdd }: { onAdd: (title: string, color: string) => void }) {
  const [open, setOpen]   = useState(false);
  const [title, setTitle] = useState("");
  const [color, setColor] = useState(SECTION_COLORS[0]);
  const inputRef = useRef<HTMLInputElement>(null);
  useEffect(() => { if (open) inputRef.current?.focus(); }, [open]);

  if (!open) return (
    <button type="button" onClick={() => setOpen(true)}
      style={{ display: "inline-flex", alignItems: "center", gap: 8, padding: "8px 18px",
        borderRadius: "var(--radius-chip)", border: "1px dashed var(--border-strong)",
        background: "transparent", color: "var(--accent)", fontSize: 13, fontWeight: 500, cursor: "pointer" }}>
      + New section
    </button>
  );

  return (
    <form onSubmit={e => { e.preventDefault(); if (title.trim()) { onAdd(title.trim(), color); setTitle(""); setOpen(false); } }}
      style={{ display: "flex", alignItems: "center", gap: 8, padding: "12px 16px",
        borderRadius: "var(--radius-card)", border: "1px solid var(--accent)",
        background: "var(--surface)", boxShadow: "var(--shadow-xs)", flexWrap: "wrap" }}>
      <span style={{ fontSize: 12, fontWeight: 600, color: "var(--text-muted)" }}>Name</span>
      <input ref={inputRef} value={title} onChange={e => setTitle(e.target.value)}
        placeholder="e.g. Broker Credentials" style={{ ...inputStyle, flex: 1, minWidth: 160 }} />
      <span style={{ fontSize: 12, color: "var(--text-muted)" }}>Color</span>
      {SECTION_COLORS.map(c => (
        <button key={c} type="button" onClick={() => setColor(c)} title={`Pick color ${c}`}
          style={{ width: 18, height: 18, borderRadius: "50%", background: c, padding: 0, flexShrink: 0,
            cursor: "pointer", border: color === c ? "2px solid var(--text-primary)" : "2px solid transparent" }} />
      ))}
      <button type="submit" style={{ padding: "5px 14px", borderRadius: "var(--radius-input)",
        border: "none", background: "var(--accent)", color: "#fff",
        fontSize: 12, fontWeight: 600, cursor: "pointer" }}>Create</button>
      <button type="button" onClick={() => setOpen(false)} style={{ padding: "5px 10px",
        borderRadius: "var(--radius-input)", border: "1px solid var(--border)",
        background: "var(--surface)", color: "var(--text-muted)", fontSize: 12, cursor: "pointer" }}>
        Cancel
      </button>
    </form>
  );
}

// ── ScreenshotsTab ────────────────────────────────────────────────────────────
function ScreenshotsTab() {
  const [screenshots, setScreenshots] = useState<Screenshot[]>(loadScreenshots);
  const [dragging, setDragging]       = useState(false);
  const [lightbox, setLightbox]       = useState<Screenshot | null>(null);
  const [editLabelId, setEditLabelId] = useState<string | null>(null);
  const [labelDraft, setLabelDraft]   = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => { saveScreenshots(screenshots); }, [screenshots]);

  const totalKB = screenshots.reduce((s, x) => s + x.sizeKB, 0);

  const processFiles = useCallback((files: FileList | null) => {
    if (!files) return;
    Array.from(files).forEach(file => {
      if (!file.type.startsWith("image/")) return;
      const reader = new FileReader();
      reader.onload = ev => {
        const dataUrl = ev.target?.result as string;
        if (!dataUrl) return;
        const sizeKB = Math.round(dataUrl.length * 0.75 / 1024);
        const shot: Screenshot = {
          id: uid(),
          dataUrl,
          label: file.name.replace(/\.[^.]+$/, ""),
          createdAt: Date.now(),
          sizeKB,
        };
        setScreenshots(prev => [shot, ...prev]);
      };
      reader.readAsDataURL(file);
    });
  }, []);

  function handleDrop(e: React.DragEvent) {
    e.preventDefault();
    setDragging(false);
    processFiles(e.dataTransfer.files);
  }

  function saveLabel(id: string) {
    setScreenshots(prev => prev.map(s => s.id === id ? { ...s, label: labelDraft.trim() || s.label } : s));
    setEditLabelId(null);
  }

  function handlePaste(e: React.ClipboardEvent) {
    const items = Array.from(e.clipboardData.items);
    const imageItems = items.filter(i => i.type.startsWith("image/"));
    imageItems.forEach(item => {
      const file = item.getAsFile();
      if (file) processFiles([file] as unknown as FileList);
    });
  }

  return (
    <div onPaste={handlePaste} tabIndex={0} style={{ outline: "none" }}>
      {/* Drop zone */}
      <div
        onDragOver={e => { e.preventDefault(); setDragging(true); }}
        onDragLeave={() => setDragging(false)}
        onDrop={handleDrop}
        onClick={() => fileInputRef.current?.click()}
        style={{
          border: `2px dashed ${dragging ? "var(--accent)" : "var(--border-strong)"}`,
          borderRadius: "var(--radius-card)",
          padding: "32px 20px",
          textAlign: "center",
          cursor: "pointer",
          background: dragging ? "rgba(26,115,232,0.04)" : "var(--surface)",
          transition: "all 0.15s ease",
          marginBottom: 20,
        }}
      >
        <div style={{ fontSize: 28, marginBottom: 8 }}>🖼️</div>
        <div style={{ fontSize: 14, fontWeight: 600, color: "var(--text-primary)", marginBottom: 4 }}>
          Drop screenshots here, click to browse, or paste (Ctrl+V)
        </div>
        <div style={{ fontSize: 12, color: "var(--text-muted)" }}>
          PNG, JPG, WEBP, GIF — stored locally in browser &nbsp;·&nbsp;
          {screenshots.length} image{screenshots.length !== 1 ? "s" : ""} &nbsp;·&nbsp;
          ~{totalKB > 1024 ? (totalKB / 1024).toFixed(1) + " MB" : totalKB + " KB"} used
        </div>
        <input ref={fileInputRef} type="file" accept="image/*" multiple
          title="Upload screenshot images"
          aria-label="Upload screenshot images"
          style={{ display: "none" }}
          onChange={e => processFiles(e.target.files)} />
      </div>

      {/* Warning if storage is getting full */}
      {totalKB > 3000 && (
        <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "8px 14px",
          borderRadius: "var(--radius-input)", background: "rgba(227,116,0,0.08)",
          border: "1px solid rgba(227,116,0,0.3)", marginBottom: 16,
          fontSize: 12, color: "var(--text-secondary)" }}>
          ⚠️ You're storing {(totalKB / 1024).toFixed(1)} MB of images in localStorage.
          Browser limit is ~5–10 MB. Delete old screenshots to free space.
        </div>
      )}

      {/* Grid */}
      {screenshots.length === 0 ? (
        <div style={{ textAlign: "center", padding: "40px 20px",
          color: "var(--text-muted)", fontSize: 13 }}>
          No screenshots yet. Drop or paste images above.
        </div>
      ) : (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))", gap: 14 }}>
          {screenshots.map(shot => (
            <div key={shot.id} style={{ background: "var(--surface)", borderRadius: "var(--radius-card)",
              border: "1px solid var(--border)", overflow: "hidden", boxShadow: "var(--shadow-xs)" }}>
              {/* Thumbnail */}
              <div style={{ position: "relative", background: "var(--surface-2)", cursor: "zoom-in" }}
                onClick={() => setLightbox(shot)}>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={shot.dataUrl} alt={shot.label}
                  style={{ width: "100%", height: 140, objectFit: "cover", display: "block" }} />
                <span style={{ position: "absolute", bottom: 4, right: 6,
                  fontSize: 10, background: "rgba(0,0,0,0.55)", color: "#fff",
                  padding: "1px 5px", borderRadius: 3 }}>
                  {shot.sizeKB} KB
                </span>
              </div>

              {/* Label */}
              <div style={{ padding: "8px 10px" }}>
                {editLabelId === shot.id ? (
                  <form onSubmit={e => { e.preventDefault(); saveLabel(shot.id); }}
                    style={{ display: "flex", gap: 4 }}>
                    <input autoFocus value={labelDraft} onChange={e => setLabelDraft(e.target.value)}
                      onBlur={() => saveLabel(shot.id)}
                      style={{ ...inputStyle, fontSize: 12, padding: "3px 6px", flex: 1 }} />
                  </form>
                ) : (
                  <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
                    <span style={{ fontSize: 12, fontWeight: 500, color: "var(--text-primary)",
                      flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
                      title={shot.label}>
                      {shot.label || "Untitled"}
                    </span>
                    <button type="button" title="Edit label" style={iconBtnStyle}
                      onClick={() => { setLabelDraft(shot.label); setEditLabelId(shot.id); }}>
                      ✏️
                    </button>
                    <button type="button" title="Download" style={iconBtnStyle}
                      onClick={() => {
                        const a = document.createElement("a");
                        a.href = shot.dataUrl;
                        a.download = shot.label || "screenshot";
                        a.click();
                      }}>⬇️</button>
                    <button type="button" title="Delete" style={{ ...iconBtnStyle, color: "var(--red)" }}
                      onClick={() => { if (window.confirm(`Delete "${shot.label}"?`)) setScreenshots(prev => prev.filter(s => s.id !== shot.id)); }}>
                      ✕
                    </button>
                  </div>
                )}
                <div style={{ fontSize: 10, color: "var(--text-muted)", marginTop: 2 }}>
                  {new Date(shot.createdAt).toLocaleString()}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Lightbox */}
      {lightbox && (
        <div onClick={() => setLightbox(null)}
          style={{ position: "fixed", inset: 0, background: "rgba(0,0,0,0.82)",
            zIndex: 9999, display: "flex", alignItems: "center", justifyContent: "center",
            cursor: "zoom-out" }}>
          <div style={{ position: "relative", maxWidth: "92vw", maxHeight: "90vh" }}
            onClick={e => e.stopPropagation()}>
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={lightbox.dataUrl} alt={lightbox.label}
              style={{ maxWidth: "92vw", maxHeight: "82vh", borderRadius: 8,
                boxShadow: "0 8px 40px rgba(0,0,0,0.5)", display: "block" }} />
            <div style={{ position: "absolute", bottom: -32, left: 0, right: 0,
              textAlign: "center", color: "#fff", fontSize: 13, fontWeight: 500 }}>
              {lightbox.label}
            </div>
            <button type="button" onClick={() => setLightbox(null)}
              style={{ position: "absolute", top: -14, right: -14, width: 28, height: 28,
                borderRadius: "50%", background: "#fff", border: "none", cursor: "pointer",
                fontSize: 14, fontWeight: 700, color: "var(--text-primary)",
                boxShadow: "var(--shadow-md)", display: "flex", alignItems: "center", justifyContent: "center" }}>
              ✕
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Main export ───────────────────────────────────────────────────────────────
type PanelTab = "notes" | "screenshots";

export default function NotePadPanel() {
  const [panelTab, setPanelTab] = useState<PanelTab>("notes");
  const [data, setData]         = useState<NotePadData>(loadNotes);
  const [search, setSearch]     = useState("");

  useEffect(() => { saveNotes(data); }, [data]);

  function updateSection(id: string, updated: NoteSection) {
    setData(d => ({ sections: d.sections.map(s => s.id === id ? updated : s) }));
  }
  function deleteSection(id: string) {
    setData(d => ({ sections: d.sections.filter(s => s.id !== id) }));
  }
  function addSection(title: string, color: string) {
    setData(d => ({ sections: [...d.sections, { id: uid(), title, color, entries: [] }] }));
  }

  const filtered = search.trim()
    ? data.sections
        .map(s => ({ ...s, entries: s.entries.filter(e =>
          e.key.toLowerCase().includes(search.toLowerCase()) ||
          e.value.toLowerCase().includes(search.toLowerCase())) }))
        .filter(s => s.title.toLowerCase().includes(search.toLowerCase()) || s.entries.length > 0)
    : data.sections;

  const totalEntries = data.sections.reduce((sum, s) => sum + s.entries.length, 0);

  return (
    <div style={{ maxWidth: 900, margin: "0 auto", padding: "20px 0 40px" }}>

      {/* Header row */}
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between",
        flexWrap: "wrap", gap: 12, marginBottom: 16 }}>
        <div>
          <h2 style={{ fontSize: 18, fontWeight: 700, color: "var(--text-primary)",
            fontFamily: "var(--font-display)", margin: 0 }}>Notepad</h2>
          <p style={{ fontSize: 12, color: "var(--text-muted)", margin: "4px 0 0" }}>
            Local browser storage · {data.sections.length} section{data.sections.length !== 1 ? "s" : ""}
            {panelTab === "notes" && ` · ${totalEntries} entr${totalEntries !== 1 ? "ies" : "y"}`}
          </p>
        </div>

        {/* Panel tabs */}
        <div style={{ display: "inline-flex", borderRadius: "var(--radius-chip)",
          border: "1px solid var(--border)", overflow: "hidden", background: "var(--surface-2)" }}>
          {(["notes", "screenshots"] as PanelTab[]).map(t => (
            <button key={t} type="button" onClick={() => setPanelTab(t)}
              style={{ padding: "6px 18px", border: "none", fontSize: 12, fontWeight: 600,
                fontFamily: "var(--font-display)", cursor: "pointer",
                background: panelTab === t ? "var(--surface)" : "transparent",
                color: panelTab === t ? "var(--text-primary)" : "var(--text-muted)",
                boxShadow: panelTab === t ? "var(--shadow-xs)" : "none",
                transition: "all 0.15s ease", textTransform: "capitalize" }}>
              {t === "notes" ? "📝 Notes & Keys" : "🖼️ Screenshots"}
            </button>
          ))}
        </div>
      </div>

      {/* Privacy notice */}
      <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "8px 14px",
        borderRadius: "var(--radius-input)", background: "rgba(26,115,232,0.06)",
        border: "1px solid rgba(26,115,232,0.18)", marginBottom: 20,
        fontSize: 12, color: "var(--text-secondary)" }}>
        <span>🔒</span>
        <span>
          Everything is stored <strong>locally in your browser only</strong> (localStorage).
          Nothing is sent to any server. Secret values are masked by default.
        </span>
      </div>

      {/* ── Notes & Keys tab ── */}
      {panelTab === "notes" && (
        <>
          {/* Search bar */}
          <div style={{ position: "relative", marginBottom: 16, display: "inline-block" }}>
            <span style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)",
              fontSize: 13, color: "var(--text-muted)", pointerEvents: "none" }}>🔍</span>
            <input value={search} onChange={e => setSearch(e.target.value)}
              placeholder="Search entries…"
              style={{ padding: "6px 12px 6px 32px", borderRadius: "var(--radius-chip)",
                border: "1px solid var(--border)", background: "var(--surface)",
                fontSize: 13, color: "var(--text-primary)", outline: "none", width: 220 }} />
            {search && (
              <button type="button" onClick={() => setSearch("")}
                style={{ position: "absolute", right: 8, top: "50%", transform: "translateY(-50%)",
                  background: "none", border: "none", cursor: "pointer",
                  color: "var(--text-muted)", fontSize: 13, lineHeight: 1 }}>✕</button>
            )}
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            {filtered.length === 0 && search ? (
              <div style={{ textAlign: "center", padding: "40px 20px",
                color: "var(--text-muted)", fontSize: 13 }}>
                No results for "{search}"
              </div>
            ) : (
              filtered.map(section => (
                <Section key={section.id} section={section}
                  onUpdate={u => updateSection(section.id, u)}
                  onDelete={() => deleteSection(section.id)} />
              ))
            )}
            {!search && <AddSectionRow onAdd={addSection} />}
          </div>
        </>
      )}

      {/* ── Screenshots tab ── */}
      {panelTab === "screenshots" && <ScreenshotsTab />}
    </div>
  );
}
