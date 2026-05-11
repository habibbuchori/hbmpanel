"use client";
import useSWR from "swr";
import { StatTile, Card } from "@/components/ui";
import { fetcher } from "@/lib/api";

type Stats = {
  cpu_count: number;
  load_avg: string;
  mem_total: number;
  mem_free: number;
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

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Overview</h1>
        <p className="text-sm text-muted">{stats?.hostname ?? "—"} · {stats?.uptime ?? ""}</p>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <StatTile label="CPU cores" value={stats?.cpu_count ?? "—"} />
        <StatTile label="Load avg"  value={(stats?.load_avg ?? "—").split(" ").slice(0, 3).join(" ")} />
        <StatTile label="Memory"    value={`${memPct}%`} hint={stats ? `${fmtBytes(memUsed)} / ${fmtBytes(stats.mem_total)}` : ""} />
        <StatTile label="Sites"     value={"—"} hint="will count from API" />
      </div>

      <Card>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-semibold uppercase tracking-wider text-muted">Services</h2>
        </div>
        <div className="divide-y divide-line">
          {(services ?? []).map((s) => (
            <div key={s.name} className="flex items-center justify-between py-2 text-sm">
              <span className="font-mono">{s.name}</span>
              <span className={s.active === "active" ? "text-green-400" : "text-muted"}>
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
