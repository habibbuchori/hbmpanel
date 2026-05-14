"use client";
import { useState } from "react";
import useSWR from "swr";
import { Card, Button } from "@/components/ui";
import { fetcher } from "@/lib/api";
import { RefreshCw } from "lucide-react";

type AuditRow = {
  id: number;
  user_id?: number;
  action: string;
  target?: string;
  detail?: string;
  created_at: string;
};

const PAGE_SIZE = 100;

export default function AuditPage() {
  const [offset, setOffset] = useState(0);
  const key = `/api/audit/?limit=${PAGE_SIZE}&offset=${offset}`;
  const { data, mutate } = useSWR<AuditRow[]>(key, fetcher, { refreshInterval: 0 });
  const [filter, setFilter] = useState("");

  const filtered = (data ?? []).filter((r) => {
    if (!filter) return true;
    const f = filter.toLowerCase();
    return (
      r.action.toLowerCase().includes(f) ||
      (r.target ?? "").toLowerCase().includes(f) ||
      (r.detail ?? "").toLowerCase().includes(f)
    );
  });

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">Audit log</h1>
        <div className="flex items-center gap-2">
          <input
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="filter…"
            className="rounded-xl bg-white/10 border border-white/15 px-3 py-1.5 text-sm"
          />
          <Button variant="outline" onClick={() => mutate()}>
            <RefreshCw size={14} /> Reload
          </Button>
        </div>
      </div>

      <Card className="p-0">
        <div className="grid grid-cols-[120px_60px_1fr_1fr] gap-2 border-b border-white/10 px-3 py-2 text-xs font-bold uppercase tracking-wider text-muted">
          <div>Time</div>
          <div>User</div>
          <div>Action</div>
          <div>Detail</div>
        </div>
        <div className="divide-y divide-white/10 font-mono text-xs">
          {filtered.length === 0 && (
            <div className="py-6 text-center text-sm text-muted">no rows</div>
          )}
          {filtered.map((r) => (
            <div key={r.id} className="grid grid-cols-[120px_60px_1fr_1fr] gap-2 px-3 py-2">
              <div className="text-muted">{new Date(r.created_at).toLocaleString()}</div>
              <div className="text-muted">{r.user_id ?? "-"}</div>
              <div className="break-all">{r.action}{r.target ? " · " + r.target : ""}</div>
              <div className="text-muted break-all">{r.detail}</div>
            </div>
          ))}
        </div>
      </Card>

      <div className="flex items-center justify-between text-xs text-muted">
        <span>offset {offset} · {filtered.length} of {data?.length ?? 0} rows shown</span>
        <div className="flex gap-2">
          <Button variant="ghost" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
            ← Newer
          </Button>
          <Button variant="ghost" disabled={!data || data.length < PAGE_SIZE} onClick={() => setOffset(offset + PAGE_SIZE)}>
            Older →
          </Button>
        </div>
      </div>
    </div>
  );
}
