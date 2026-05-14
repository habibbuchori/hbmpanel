"use client";
import { useState } from "react";
import useSWR, { mutate } from "swr";
import { Button, Card } from "@/components/ui";
import { PremiumGate } from "@/components/premium";
import { api, fetcher } from "@/lib/api";
import { Download, Trash2, RotateCw, Archive, Database } from "lucide-react";

type Backup = {
  id: number;
  kind: "site" | "db";
  target: string;
  label: string;
  file_path: string;
  size_bytes: number;
  created_at: string;
};
type Site = { id: number; domain: string };

const fmtSize = (n: number) =>
  n >= 1e9 ? (n / 1e9).toFixed(2) + " GB" :
  n >= 1e6 ? (n / 1e6).toFixed(1) + " MB" :
  n >= 1e3 ? (n / 1e3).toFixed(1) + " KB" : n + " B";

export default function BackupsPage() {
  const { data: backups, error } = useSWR<Backup[]>("/api/backup/", fetcher);
  const { data: sites } = useSWR<Site[]>("/api/sites/", fetcher);
  const { data: dbs } = useSWR<string[]>("/api/postgres/databases", fetcher);

  const [siteId, setSiteId] = useState<number>(0);
  const [dbName, setDbName] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);

  if (error) return <PremiumGate error={error} />;

  async function backupSite() {
    if (!siteId) return;
    setBusy("site");
    setErr(null);
    try {
      await api(`/api/backup/site/${siteId}`, { method: "POST" });
      mutate("/api/backup/");
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(null);
    }
  }

  async function backupDB() {
    if (!dbName) return;
    setBusy("db");
    setErr(null);
    try {
      await api(`/api/backup/db/${encodeURIComponent(dbName)}`, { method: "POST" });
      mutate("/api/backup/");
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(null);
    }
  }

  async function restore(b: Backup) {
    if (!confirm(`Restore ${b.label}? Ini akan menimpa data existing.`)) return;
    setBusy("restore-" + b.id);
    try {
      await api(`/api/backup/${b.id}/restore`, { method: "POST" });
      alert("restore selesai");
    } catch (e) {
      alert(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(null);
    }
  }

  async function remove(b: Backup) {
    if (!confirm(`Hapus backup ${b.label}?`)) return;
    await api(`/api/backup/${b.id}`, { method: "DELETE" });
    mutate("/api/backup/");
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Backups</h1>
        <p className="text-sm text-muted">
          Disimpan di <code>/var/lib/hbmpanel/backups/</code>. Site backup = tar.gz dari root_path.
          DB backup = pg_dump.gz.
        </p>
      </div>

      <div className="grid md:grid-cols-2 gap-4">
        <Card>
          <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">
            <Archive size={14} className="inline mr-1" /> Site backup
          </h2>
          <div className="flex gap-2">
            <select
              value={siteId || ""}
              onChange={(e) => setSiteId(Number(e.target.value))}
              className="flex-1 rounded-xl bg-white/10 border border-white/15 px-3 py-2 text-sm"
            >
              <option value="">— pilih site —</option>
              {(sites ?? []).map((s) => <option key={s.id} value={s.id}>{s.domain}</option>)}
            </select>
            <Button onClick={backupSite} disabled={!siteId || !!busy}>
              {busy === "site" ? "Working…" : "Backup"}
            </Button>
          </div>
        </Card>

        <Card>
          <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">
            <Database size={14} className="inline mr-1" /> DB backup
          </h2>
          <div className="flex gap-2">
            <select
              value={dbName}
              onChange={(e) => setDbName(e.target.value)}
              className="flex-1 rounded-xl bg-white/10 border border-white/15 px-3 py-2 text-sm"
            >
              <option value="">— pilih database —</option>
              {(dbs ?? []).map((d) => <option key={d} value={d}>{d}</option>)}
            </select>
            <Button onClick={backupDB} disabled={!dbName || !!busy}>
              {busy === "db" ? "Working…" : "Backup"}
            </Button>
          </div>
        </Card>
      </div>

      {err && <Card><div className="text-xs text-red-300">{err}</div></Card>}

      <Card>
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">All backups</h2>
        <div className="divide-y divide-white/10">
          {(backups ?? []).map((b) => (
            <div key={b.id} className="flex flex-col md:flex-row md:items-center md:justify-between gap-2 py-3">
              <div className="min-w-0">
                <div className="flex items-center gap-2 text-sm">
                  {b.kind === "site" ? <Archive size={14} /> : <Database size={14} />}
                  <span className="truncate font-mono">{b.label}</span>
                </div>
                <div className="text-xs text-muted">
                  {fmtSize(b.size_bytes)} · {new Date(b.created_at).toLocaleString()} · {b.file_path}
                </div>
              </div>
              <div className="flex items-center gap-2">
                <a
                  className="inline-flex items-center gap-2 rounded-xl border border-white/15 bg-white/5 px-3 py-1.5 text-sm hover:bg-white/10"
                  href={`/api/backup/${b.id}/download`}
                >
                  <Download size={14} /> Download
                </a>
                <Button variant="outline" onClick={() => restore(b)} disabled={busy === "restore-" + b.id}>
                  <RotateCw size={14} /> {busy === "restore-" + b.id ? "Restoring…" : "Restore"}
                </Button>
                <Button variant="danger" onClick={() => remove(b)}>
                  <Trash2 size={14} /> Delete
                </Button>
              </div>
            </div>
          ))}
          {backups && backups.length === 0 && (
            <div className="py-6 text-center text-sm text-muted">Belum ada backup.</div>
          )}
        </div>
      </Card>
    </div>
  );
}
