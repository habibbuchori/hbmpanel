"use client";
import { useState } from "react";
import useSWR from "swr";
import { Button, Card, Input } from "@/components/ui";
import { api, fetcher } from "@/lib/api";
import { Play, Sparkles } from "lucide-react";

type LaravelSite = { id: number; domain: string; root_path: string; project_dir: string };
type ArtisanResp = { output: string; exit_code: number };

const presets: { name: string; label: string; desc: string }[] = [
  { name: "migrate",        label: "Migrate",        desc: "php artisan migrate --force" },
  { name: "migrate-fresh",  label: "Migrate fresh",  desc: "drop all + re-migrate (data loss!)" },
  { name: "seed",           label: "Seed",           desc: "php artisan db:seed --force" },
  { name: "cache-clear",    label: "Clear caches",   desc: "cache + config + route + view clear" },
  { name: "optimize",       label: "Optimize",       desc: "php artisan optimize" },
  { name: "optimize-clear", label: "Optimize clear", desc: "php artisan optimize:clear" },
  { name: "queue-restart",  label: "Queue restart",  desc: "signal workers to restart" },
  { name: "storage-link",   label: "Storage link",   desc: "php artisan storage:link" },
];

export default function LaravelPage() {
  const { data: sites } = useSWR<LaravelSite[]>("/api/laravel/sites", fetcher);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [customCmd, setCustomCmd] = useState("");
  const [result, setResult] = useState<ArtisanResp | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const selected = sites?.find((s) => s.id === selectedId) ?? sites?.[0] ?? null;
  const id = selected?.id;

  async function runPreset(name: string) {
    if (!id) return;
    setBusy(name);
    setErr(null);
    try {
      const r = await api<ArtisanResp>(`/api/laravel/${id}/preset/${name}`, { method: "POST" });
      setResult(r);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(null);
    }
  }

  async function runCustom(e: React.FormEvent) {
    e.preventDefault();
    if (!id || !customCmd.trim()) return;
    setBusy("custom");
    setErr(null);
    try {
      const r = await api<ArtisanResp>(`/api/laravel/${id}/artisan`, {
        method: "POST",
        body: JSON.stringify({ cmd: customCmd }),
      });
      setResult(r);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Laravel</h1>
        <p className="text-sm text-muted">
          Jalankan artisan command langsung dari panel. Hanya site PHP yang punya file <code>artisan</code> di project root.
        </p>
      </div>

      {sites && sites.length === 0 ? (
        <Card>
          <div className="py-6 text-center text-sm text-muted">
            Belum ada Laravel site terdeteksi.
          </div>
        </Card>
      ) : (
        <>
          <Card>
            <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">Site</h2>
            <select
              value={id ?? ""}
              onChange={(e) => setSelectedId(Number(e.target.value))}
              className="w-full rounded-xl bg-white/10 border border-white/15 px-3 py-2 text-sm"
            >
              {(sites ?? []).map((s) => (
                <option key={s.id} value={s.id}>
                  {s.domain} — {s.project_dir}
                </option>
              ))}
            </select>
          </Card>

          <Card>
            <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">Presets</h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
              {presets.map((p) => (
                <Button
                  key={p.name}
                  variant="outline"
                  disabled={!id || !!busy}
                  onClick={() => runPreset(p.name)}
                  title={p.desc}
                >
                  <Sparkles size={14} />
                  {busy === p.name ? "Running…" : p.label}
                </Button>
              ))}
            </div>
          </Card>

          <Card>
            <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">Custom artisan</h2>
            <form onSubmit={runCustom} className="flex gap-2">
              <Input
                placeholder="route:list, tinker --execute='...', dll"
                value={customCmd}
                onChange={(e) => setCustomCmd(e.target.value)}
              />
              <Button type="submit" disabled={!id || !!busy}>
                <Play size={14} /> {busy === "custom" ? "Running…" : "Run"}
              </Button>
            </form>
          </Card>

          {(result || err) && (
            <Card className="p-0">
              <div className="px-4 py-3 border-b border-white/10 flex items-center justify-between">
                <h2 className="text-sm font-semibold uppercase tracking-wider text-muted">Output</h2>
                {result && (
                  <span className={result.exit_code === 0 ? "text-xs text-mint" : "text-xs text-red-300"}>
                    exit {result.exit_code}
                  </span>
                )}
              </div>
              <pre className="scrollbar max-h-80 overflow-auto bg-black/30 p-4 font-mono text-xs leading-snug whitespace-pre-wrap">
                {err ?? result?.output ?? ""}
              </pre>
            </Card>
          )}
        </>
      )}
    </div>
  );
}
