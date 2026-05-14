"use client";
import useSWR from "swr";
import { StatTile, Card } from "@/components/ui";
import { fetcher } from "@/lib/api";

type Stats = {
  cpu_count: number;
  load_avg: string;
  mem_total: number;
  mem_free: number;
  disk_total: number;
  disk_free: number;
  site_count: number;
  pm2_count: number;
  redis_used: string;
  postgres_status: string;
  uptime: string;
  hostname: string;
};
type Service = { name: string; active: string; sub: string };

const fmtBytes = (n: number) =>
  n >= 1e9 ? (n / 1e9).toFixed(1) + " GB" : (n / 1e6).toFixed(0) + " MB";

export default function OverviewPage() {
  const { data: stats } = useSWR<Stats>("/api/system/stats", fetcher, { refreshInterval: 5000 });
  const { data: services } = useSWR<Service[]>("/api/system/services", fetcher, { refreshInterval: 10000 });

  const memUsed = stats ? stats.mem_total - stats.mem_free : 0;
  const memPct = stats && stats.mem_total > 0 ? Math.round((memUsed / stats.mem_total) * 100) : 0;
  const diskUsed = stats ? stats.disk_total - stats.disk_free : 0;
  const diskPct = stats && stats.disk_total > 0 ? Math.round((diskUsed / stats.disk_total) * 100) : 0;

  return (
    <div className="space-y-6">
      <div className="rounded-3xl border border-white/10 bg-gradient-to-r from-brand/15 via-candy/15 to-sunny/10 p-6 shadow-xl shadow-black/10">
        <div className="text-sm font-bold uppercase tracking-wider text-brand">✨ Server vibes</div>
        <h1 className="mt-1 text-3xl font-black">Overview</h1>
        <p className="text-sm text-muted">{stats?.hostname ?? "—"} · {stats?.uptime ?? ""}</p>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <StatTile label="🧠 CPU cores" value={stats?.cpu_count ?? "—"} />
        <StatTile label="⚡ Load avg"  value={(stats?.load_avg ?? "—").split(" ").slice(0, 3).join(" ")} />
        <StatTile label="🌈 Memory"    value={`${memPct}%`} hint={stats ? `${fmtBytes(memUsed)} / ${fmtBytes(stats.mem_total)}` : ""} />
        <StatTile label="💾 Disk"      value={`${diskPct}%`} hint={stats ? `${fmtBytes(diskUsed)} / ${fmtBytes(stats.disk_total)}` : ""} />
        <StatTile label="🌍 Sites"     value={stats?.site_count ?? "—"} hint="caddy vhosts" />
        <StatTile label="🚀 PM2 apps"  value={stats?.pm2_count ?? "—"} hint="node processes" />
        <StatTile label="🟥 Redis"     value={stats?.redis_used || "—"} hint="memory used" />
        <StatTile
          label="🐘 Postgres"
          value={stats?.postgres_status || "—"}
          hint={stats?.postgres_status === "active" ? "running" : "check service"}
        />
      </div>

      <Card>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-bold uppercase tracking-wider text-muted">Services playground</h2>
        </div>
        <div className="divide-y divide-white/10">
          {(services ?? []).map((s) => (
            <div key={s.name} className="flex items-center justify-between py-2 text-sm">
              <span className="font-mono">{s.name}</span>
              <span className={s.active === "active" ? "text-mint" : "text-muted"}>
                {s.active} · {s.sub}
              </span>
            </div>
          ))}
          {!services && <div className="text-sm text-muted py-2">loading…</div>}
        </div>
      </Card>
    </div>
  );
}
