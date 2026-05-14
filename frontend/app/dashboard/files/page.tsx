"use client";
import { useEffect, useRef, useState } from "react";
import useSWR, { mutate } from "swr";
import { Button, Card, Input } from "@/components/ui";
import { PremiumGate } from "@/components/premium";
import { api, fetcher } from "@/lib/api";
import {
  ChevronRight, Folder, File as FileIcon, Upload, FolderPlus, Trash2,
  Download, Edit3, Archive, X, RefreshCw,
} from "lucide-react";

type Entry = {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
  mode: string;
  mtime: string;
};

type ListResp = { path: string; entries: Entry[] };

const fmtSize = (n: number) =>
  n >= 1e9 ? (n / 1e9).toFixed(1) + " GB" :
  n >= 1e6 ? (n / 1e6).toFixed(1) + " MB" :
  n >= 1e3 ? (n / 1e3).toFixed(1) + " KB" : n + " B";

export default function FilesPage() {
  const [path, setPath] = useState("/var/www");
  const [pathInput, setPathInput] = useState("/var/www");
  const { data, error } = useSWR<ListResp>(`/api/files/list?path=${encodeURIComponent(path)}`, fetcher);
  const fileInput = useRef<HTMLInputElement>(null);
  const [editor, setEditor] = useState<Entry | null>(null);
  const [busy, setBusy] = useState(false);

  if (error) return <PremiumGate error={error} />;

  function go(p: string) {
    setPath(p);
    setPathInput(p);
  }

  async function mkdir() {
    const name = prompt("Nama folder baru:");
    if (!name) return;
    const target = path.replace(/\/$/, "") + "/" + name;
    await api("/api/files/mkdir", { method: "POST", body: JSON.stringify({ path: target }) });
    mutate(`/api/files/list?path=${encodeURIComponent(path)}`);
  }

  async function remove(e: Entry) {
    if (!confirm(`Hapus ${e.is_dir ? "folder" : "file"} "${e.name}"?`)) return;
    await api(`/api/files/?path=${encodeURIComponent(e.path)}`, { method: "DELETE" });
    mutate(`/api/files/list?path=${encodeURIComponent(path)}`);
  }

  async function rename(e: Entry) {
    const next = prompt("Nama baru:", e.name);
    if (!next || next === e.name) return;
    const to = path.replace(/\/$/, "") + "/" + next;
    await api("/api/files/rename", { method: "POST", body: JSON.stringify({ from: e.path, to }) });
    mutate(`/api/files/list?path=${encodeURIComponent(path)}`);
  }

  async function chmod(e: Entry) {
    const mode = prompt("Mode octal (mis. 755):", e.mode.replace(/^0+o?/, ""));
    if (!mode) return;
    await api("/api/files/chmod", { method: "POST", body: JSON.stringify({ path: e.path, mode }) });
    mutate(`/api/files/list?path=${encodeURIComponent(path)}`);
  }

  async function extract(e: Entry) {
    if (!/\.(zip|tar\.gz|tgz|tar)$/i.test(e.name)) {
      alert("File bukan archive .zip / .tar / .tar.gz");
      return;
    }
    if (!confirm(`Ekstrak ${e.name} ke ${path}?`)) return;
    setBusy(true);
    try {
      await api("/api/files/extract", { method: "POST", body: JSON.stringify({ path: e.path, dest: path }) });
      mutate(`/api/files/list?path=${encodeURIComponent(path)}`);
    } finally {
      setBusy(false);
    }
  }

  async function upload(files: FileList | null) {
    if (!files || files.length === 0) return;
    setBusy(true);
    try {
      const form = new FormData();
      Array.from(files).forEach((f) => form.append("files", f));
      const res = await fetch(`/api/files/upload?path=${encodeURIComponent(path)}`, {
        method: "POST",
        credentials: "include",
        headers: { "X-Requested-With": "hbmpanel" },
        body: form,
      });
      if (!res.ok) alert("upload failed: " + (await res.text()));
      mutate(`/api/files/list?path=${encodeURIComponent(path)}`);
    } finally {
      setBusy(false);
    }
  }

  const breadcrumbs = path.split("/").filter(Boolean);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">Files</h1>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={() => mutate(`/api/files/list?path=${encodeURIComponent(path)}`)}>
            <RefreshCw size={14} /> Reload
          </Button>
          <Button variant="outline" onClick={mkdir}>
            <FolderPlus size={14} /> New folder
          </Button>
          <Button onClick={() => fileInput.current?.click()} disabled={busy}>
            <Upload size={14} /> {busy ? "Working…" : "Upload"}
          </Button>
          <input
            ref={fileInput}
            type="file"
            multiple
            className="hidden"
            onChange={(e) => upload(e.target.files)}
          />
        </div>
      </div>

      <Card>
        <form
          onSubmit={(e) => { e.preventDefault(); go(pathInput); }}
          className="flex gap-2 mb-3"
        >
          <Input
            value={pathInput}
            onChange={(e) => setPathInput(e.target.value)}
            placeholder="/absolute/path"
            className="font-mono"
          />
          <Button type="submit" variant="outline">Go</Button>
        </form>

        <div className="mb-3 flex flex-wrap items-center gap-1 text-xs font-mono text-muted">
          <button onClick={() => go("/")} className="hover:text-fg">/</button>
          {breadcrumbs.map((seg, i) => {
            const target = "/" + breadcrumbs.slice(0, i + 1).join("/");
            return (
              <span key={target} className="flex items-center gap-1">
                <ChevronRight size={12} />
                <button onClick={() => go(target)} className="hover:text-fg">{seg}</button>
              </span>
            );
          })}
        </div>

        <div className="divide-y divide-white/10">
          {path !== "/" && (
            <button
              onClick={() => go(path.replace(/\/[^/]+$/, "") || "/")}
              className="flex w-full items-center gap-2 py-2 text-sm text-muted hover:text-fg"
            >
              <Folder size={14} /> ..
            </button>
          )}
          {(data?.entries ?? []).map((e) => (
            <div key={e.path} className="flex items-center justify-between py-2 text-sm">
              <div className="flex min-w-0 items-center gap-2">
                {e.is_dir
                  ? <Folder size={14} className="shrink-0 text-brand" />
                  : <FileIcon size={14} className="shrink-0 text-muted" />}
                {e.is_dir ? (
                  <button onClick={() => go(e.path)} className="truncate font-mono hover:text-brand">
                    {e.name}
                  </button>
                ) : (
                  <span className="truncate font-mono">{e.name}</span>
                )}
              </div>
              <div className="flex items-center gap-3">
                <span className="text-xs text-muted tabular-nums">
                  {e.is_dir ? "" : fmtSize(e.size)} · {e.mode}
                </span>
                <div className="flex items-center gap-1">
                  {!e.is_dir && (
                    <button title="Edit" onClick={() => setEditor(e)}
                      className="rounded-lg p-1.5 text-muted hover:bg-white/10 hover:text-fg">
                      <Edit3 size={14} />
                    </button>
                  )}
                  {!e.is_dir && /\.(zip|tar\.gz|tgz|tar)$/i.test(e.name) && (
                    <button title="Extract" onClick={() => extract(e)}
                      className="rounded-lg p-1.5 text-muted hover:bg-white/10 hover:text-fg">
                      <Archive size={14} />
                    </button>
                  )}
                  {!e.is_dir && (
                    <a
                      title="Download"
                      href={`/api/files/download?path=${encodeURIComponent(e.path)}`}
                      className="rounded-lg p-1.5 text-muted hover:bg-white/10 hover:text-fg"
                    >
                      <Download size={14} />
                    </a>
                  )}
                  <button title="Rename" onClick={() => rename(e)}
                    className="rounded-lg p-1.5 text-muted hover:bg-white/10 hover:text-fg text-xs px-2">
                    ren
                  </button>
                  <button title="Chmod" onClick={() => chmod(e)}
                    className="rounded-lg p-1.5 text-muted hover:bg-white/10 hover:text-fg text-xs px-2">
                    chmod
                  </button>
                  <button title="Delete" onClick={() => remove(e)}
                    className="rounded-lg p-1.5 text-red-300 hover:bg-red-500/15">
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            </div>
          ))}
          {data && data.entries.length === 0 && (
            <div className="py-6 text-center text-sm text-muted">Folder kosong.</div>
          )}
        </div>
      </Card>

      {editor && <FileEditor entry={editor} onClose={() => setEditor(null)} />}
    </div>
  );
}

function FileEditor({ entry, onClose }: { entry: Entry; onClose: () => void }) {
  const [content, setContent] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [original, setOriginal] = useState<string | null>(null);

  useEffect(() => {
    let aborted = false;
    fetch(`/api/files/read?path=${encodeURIComponent(entry.path)}`, {
      credentials: "include",
      headers: { "X-Requested-With": "hbmpanel" },
    })
      .then(async (r) => {
        if (!r.ok) throw new Error(await r.text());
        return r.text();
      })
      .then((t) => {
        if (aborted) return;
        setContent(t);
        setOriginal(t);
      })
      .catch((e) => { if (!aborted) setErr(e.message); });
    return () => { aborted = true; };
  }, [entry.path]);

  async function save() {
    if (content === null) return;
    setSaving(true);
    setErr(null);
    try {
      await api("/api/files/write", { method: "POST", body: JSON.stringify({ path: entry.path, content }) });
      onClose();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 px-4 backdrop-blur-sm" onClick={onClose}>
      <div
        className="flex max-h-[90vh] w-full max-w-4xl flex-col gap-3 rounded-3xl border border-white/15 bg-panel p-5 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between">
          <h3 className="font-mono text-sm">{entry.path}</h3>
          <button onClick={onClose} className="rounded-lg p-1 text-muted hover:bg-white/10 hover:text-fg">
            <X size={16} />
          </button>
        </div>
        {err && <div className="text-xs text-red-300">{err}</div>}
        {content === null && !err && <div className="text-sm text-muted">loading…</div>}
        {content !== null && (
          <textarea
            value={content}
            onChange={(e) => setContent(e.target.value)}
            className="h-[60vh] w-full rounded-xl bg-black/30 border border-white/10 p-3 font-mono text-xs"
          />
        )}
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button onClick={save} disabled={saving || content === null || content === original}>
            {saving ? "Saving…" : "Save"}
          </Button>
        </div>
      </div>
    </div>
  );
}
