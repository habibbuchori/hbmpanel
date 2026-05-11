"use client";
import { useState } from "react";
import useSWR, { mutate } from "swr";
import { Button, Card, Input } from "@/components/ui";
import { api, fetcher } from "@/lib/api";
import { Trash2 } from "lucide-react";

type Site = {
  id: number; domain: string; type: string; root_path: string;
  ssl: boolean; status: string; node_port?: number;
};

export default function SitesPage() {
  const { data: sites } = useSWR<Site[]>("/api/sites/", fetcher);
  const [form, setForm] = useState({
    domain: "", type: "php", root_path: "/var/www/html", node_port: 0, ssl: true,
  });
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      await api("/api/sites/", { method: "POST", body: JSON.stringify(form) });
      setForm({ domain: "", type: "php", root_path: "/var/www/html", node_port: 0, ssl: true });
      mutate("/api/sites/");
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(false);
    }
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
        <form onSubmit={create} className="grid grid-cols-1 md:grid-cols-4 gap-3">
          <Input placeholder="domain (example.com)"
            value={form.domain} onChange={(e) => setForm({ ...form, domain: e.target.value })} />
          <select className="rounded-md bg-bg border border-line px-3 py-2 text-sm"
            value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
            <option value="php">PHP / Laravel</option>
            <option value="static">Static</option>
            <option value="node">Node.js (reverse proxy)</option>
            <option value="proxy">Proxy</option>
          </select>
          {(form.type === "php" || form.type === "static") ? (
            <Input placeholder="root path"
              value={form.root_path} onChange={(e) => setForm({ ...form, root_path: e.target.value })} />
          ) : (
            <Input type="number" placeholder="node port"
              value={form.node_port || ""}
              onChange={(e) => setForm({ ...form, node_port: Number(e.target.value) })} />
          )}
          <Button type="submit" disabled={busy}>{busy ? "Creating…" : "Create"}</Button>
        </form>
        {err && <div className="mt-2 text-xs text-red-400">{err}</div>}
      </Card>

      <Card>
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">All sites</h2>
        <div className="divide-y divide-line">
          {(sites ?? []).map((s) => (
            <div key={s.id} className="flex items-center justify-between py-3">
              <div>
                <div className="font-mono">{s.domain}</div>
                <div className="text-xs text-muted">
                  {s.type} · {s.root_path || `:${s.node_port}`} · {s.status}
                </div>
              </div>
              <Button variant="danger" onClick={() => remove(s.id)}>
                <Trash2 size={14} /> Delete
              </Button>
            </div>
          ))}
          {sites && sites.length === 0 && (
            <div className="py-6 text-center text-sm text-muted">Belum ada site.</div>
          )}
        </div>
      </Card>
    </div>
  );
}
