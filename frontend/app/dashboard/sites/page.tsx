"use client";
import { useEffect, useRef, useState } from "react";
import useSWR, { mutate } from "swr";
import { Button, Card, Input } from "@/components/ui";
import { api, fetcher } from "@/lib/api";
import { Trash2, Power, PowerOff, RefreshCw, FileText, KeyRound, X, Code2 } from "lucide-react";

type Site = {
  id: number;
  domain: string;
  type: string;
  root_path: string;
  php_version?: string;
  ssl: boolean;
  status: string;
  node_port?: number;
};

export default function SitesPage() {
  const { data: sites } = useSWR<Site[]>("/api/sites/", fetcher);
  const [form, setForm] = useState({
    domain: "", type: "php", root_path: "", php_version: "8.4", node_port: 0, ssl: true,
  });
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [envEditor, setEnvEditor] = useState<Site | null>(null);
  const [logViewer, setLogViewer] = useState<Site | null>(null);
  const [caddyEditor, setCaddyEditor] = useState<Site | null>(null);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      await api("/api/sites/", { method: "POST", body: JSON.stringify(form) });
      setForm({ domain: "", type: "php", root_path: "", php_version: "8.4", node_port: 0, ssl: true });
      mutate("/api/sites/");
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(false);
    }
  }

  async function action(id: number, kind: "restart" | "enable" | "disable") {
    await api(`/api/sites/${id}/${kind}`, { method: "POST" });
    mutate("/api/sites/");
  }

  async function remove(id: number) {
    if (!confirm("Hapus site ini?")) return;
    await api(`/api/sites/${id}`, { method: "DELETE" });
    mutate("/api/sites/");
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">Sites</h1>

      <Card>
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">Create site</h2>
        <form onSubmit={create} className="space-y-3">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <Input placeholder="domain (example.com)"
              value={form.domain} onChange={(e) => setForm({ ...form, domain: e.target.value })} />
            <select className="rounded-xl bg-white/10 border border-white/15 px-3 py-2 text-sm"
              value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
              <option value="php">PHP / Laravel</option>
              <option value="static">Static</option>
              <option value="node">Node.js (reverse proxy)</option>
              <option value="proxy">Proxy</option>
            </select>
            {(form.type === "php" || form.type === "static") ? (
              <div>
                <Input placeholder="/var/www/namaproject/public"
                  value={form.root_path} onChange={(e) => setForm({ ...form, root_path: e.target.value })} />
                {form.type === "php" && (
                  <p className="text-xs text-muted mt-1">Laravel: /var/www/namaproject/public</p>
                )}
              </div>
            ) : (
              <Input type="number" placeholder="node port"
                value={form.node_port || ""}
                onChange={(e) => setForm({ ...form, node_port: Number(e.target.value) })} />
            )}
          </div>
          <div className="flex flex-wrap items-center gap-3">
            {form.type === "php" && (
              <select
                className="rounded-xl bg-white/10 border border-white/15 px-3 py-2 text-sm"
                value={form.php_version}
                onChange={(e) => setForm({ ...form, php_version: e.target.value })}
              >
                <option value="8.1">PHP 8.1</option>
                <option value="8.2">PHP 8.2</option>
                <option value="8.3">PHP 8.3</option>
                <option value="8.4">PHP 8.4</option>
              </select>
            )}
            <label className="flex items-center gap-2 text-sm cursor-pointer select-none">
              <input
                type="checkbox"
                checked={form.ssl}
                onChange={(e) => setForm({ ...form, ssl: e.target.checked })}
                className="rounded"
              />
              SSL
            </label>
            <Button type="submit" disabled={busy}>{busy ? "Creating…" : "Create"}</Button>
          </div>
        </form>
        {err && <div className="mt-2 text-xs text-red-400">{err}</div>}
      </Card>

      <Card>
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">All sites</h2>
        <div className="divide-y divide-white/10">
          {(sites ?? []).map((s) => (
            <div key={s.id} className="flex flex-col md:flex-row md:items-center md:justify-between gap-3 py-3">
              <div>
                <div className="font-mono">{s.domain}</div>
                <div className="text-xs text-muted">
                  {s.type} · {s.root_path || `:${s.node_port}`} ·{" "}
                  <span className={s.status === "active" ? "text-mint" : "text-muted"}>{s.status}</span>
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Button variant="outline" onClick={() => action(s.id, "restart")}>
                  <RefreshCw size={14} /> Restart
                </Button>
                {s.status === "active" ? (
                  <Button variant="outline" onClick={() => action(s.id, "disable")}>
                    <PowerOff size={14} /> Disable
                  </Button>
                ) : (
                  <Button variant="outline" onClick={() => action(s.id, "enable")}>
                    <Power size={14} /> Enable
                  </Button>
                )}
                <Button variant="ghost" onClick={() => setEnvEditor(s)}>
                  <KeyRound size={14} /> Env
                </Button>
                <Button variant="ghost" onClick={() => setLogViewer(s)}>
                  <FileText size={14} /> Logs
                </Button>
                <Button variant="ghost" onClick={() => setCaddyEditor(s)}>
                  <Code2 size={14} /> Caddy
                </Button>
                <Button variant="danger" onClick={() => remove(s.id)}>
                  <Trash2 size={14} /> Delete
                </Button>
              </div>
            </div>
          ))}
          {sites && sites.length === 0 && (
            <div className="py-6 text-center text-sm text-muted">Belum ada site.</div>
          )}
        </div>
      </Card>

      {envEditor && <EnvEditor site={envEditor} onClose={() => setEnvEditor(null)} />}
      {logViewer && <LogModal site={logViewer} onClose={() => setLogViewer(null)} />}
      {caddyEditor && <CaddyModal site={caddyEditor} onClose={() => setCaddyEditor(null)} />}
    </div>
  );
}

function EnvEditor({ site, onClose }: { site: Site; onClose: () => void }) {
  const { data: env } = useSWR<Record<string, string>>(`/api/sites/${site.id}/env`, fetcher);
  const [text, setText] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const current = text ?? (env ? Object.entries(env).map(([k, v]) => `${k}=${v}`).join("\n") : "");

  async function save() {
    setSaving(true);
    setErr(null);
    try {
      const obj: Record<string, string> = {};
      for (const line of current.split("\n")) {
        const t = line.trim();
        if (!t || t.startsWith("#")) continue;
        const eq = t.indexOf("=");
        if (eq < 1) continue;
        obj[t.slice(0, eq).trim()] = t.slice(eq + 1).trim();
      }
      await api(`/api/sites/${site.id}/env`, { method: "PUT", body: JSON.stringify(obj) });
      mutate(`/api/sites/${site.id}/env`);
      onClose();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal title={`Env vars · ${site.domain}`} onClose={onClose}>
      <textarea
        value={current}
        onChange={(e) => setText(e.target.value)}
        placeholder="KEY=value&#10;ANOTHER=value"
        className="h-64 w-full rounded-xl bg-white/10 border border-white/15 px-3 py-2 font-mono text-xs"
      />
      {err && <div className="text-xs text-red-300">{err}</div>}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>Cancel</Button>
        <Button onClick={save} disabled={saving}>{saving ? "Saving…" : "Save"}</Button>
      </div>
    </Modal>
  );
}

function LogModal({ site, onClose }: { site: Site; onClose: () => void }) {
  const [lines, setLines] = useState<string[]>([]);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${location.host}/api/ws/sitelog/${site.id}`);
    ws.onmessage = (e) => {
      setLines((prev) => [...prev.slice(-2000), e.data as string]);
      requestAnimationFrame(() => {
        boxRef.current?.scrollTo(0, boxRef.current.scrollHeight);
      });
    };
    return () => ws.close();
  }, [site.id]);

  return (
    <Modal title={`Logs · ${site.domain}`} onClose={onClose}>
      <div
        ref={boxRef}
        className="scrollbar h-80 overflow-auto rounded-xl bg-black/30 p-3 font-mono text-xs leading-snug"
      >
        {lines.length === 0 && <div className="text-muted">connecting…</div>}
        {lines.map((l, i) => <div key={i} className="whitespace-pre">{l}</div>)}
      </div>
    </Modal>
  );
}

function CaddyModal({ site, onClose }: { site: Site; onClose: () => void }) {
  const { data } = useSWR<{ content: string }>(`/api/sites/${site.id}/caddy`, fetcher);
  const [text, setText] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const current = text ?? data?.content ?? "";

  async function save() {
    setSaving(true);
    setErr(null);
    try {
      await api(`/api/sites/${site.id}/caddy`, {
        method: "PUT",
        body: JSON.stringify({ content: current }),
      });
      onClose();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal title={`Caddy config · ${site.domain}`} onClose={onClose}>
      <textarea
        value={current}
        onChange={(e) => setText(e.target.value)}
        className="h-80 w-full rounded-xl bg-black/30 border border-white/15 px-3 py-2 font-mono text-xs"
        spellCheck={false}
      />
      {err && <div className="text-xs text-red-300">{err}</div>}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>Cancel</Button>
        <Button onClick={save} disabled={saving}>{saving ? "Saving…" : "Save"}</Button>
      </div>
    </Modal>
  );
}

function Modal({ title, children, onClose }: { title: string; children: React.ReactNode; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 px-4 backdrop-blur-sm" onClick={onClose}>
      <div
        className="w-full max-w-2xl space-y-3 rounded-3xl border border-white/15 bg-panel p-5 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-bold uppercase tracking-wider text-muted">{title}</h3>
          <button onClick={onClose} className="rounded-lg p-1 text-muted hover:bg-white/10 hover:text-fg">
            <X size={16} />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}
