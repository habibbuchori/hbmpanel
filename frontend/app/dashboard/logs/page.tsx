"use client";
import { useEffect, useMemo, useRef, useState } from "react";
import useSWR from "swr";
import { Button, Card, Input } from "@/components/ui";
import { api, fetcher } from "@/lib/api";
import { Search, Download, RefreshCw, Radio } from "lucide-react";

type Source = { key: string; label: string; kind: "journal" | "file" | "pm2" };

const liveTargets = ["panel", "caddy", "php", "postgres"] as const;
type LiveTarget = (typeof liveTargets)[number];

export default function LogsPage() {
  const { data: sources } = useSWR<Source[]>("/api/logs/sources", fetcher);
  const [source, setSource] = useState<string>("panel");
  const [mode, setMode] = useState<"live" | "search">("live");
  const [query, setQuery] = useState("");
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);

  // Reset when source changes.
  useEffect(() => {
    setBody("");
  }, [source, mode]);

  // Live tail (WS) only works for the legacy whitelisted targets.
  const liveTarget: LiveTarget | null = useMemo(() => {
    return (liveTargets as readonly string[]).includes(source) ? (source as LiveTarget) : null;
  }, [source]);

  useEffect(() => {
    if (mode !== "live" || !liveTarget) return;
    setBody("");
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${location.host}/api/ws/logs/${liveTarget}`);
    ws.onmessage = (e) => {
      setBody((prev) => prev + (e.data as string) + "\n");
      requestAnimationFrame(() => {
        boxRef.current?.scrollTo(0, boxRef.current.scrollHeight);
      });
    };
    return () => ws.close();
  }, [mode, liveTarget]);

  async function loadTail() {
    setBusy(true);
    try {
      const text = await fetch(`/api/logs/tail?source=${encodeURIComponent(source)}&lines=500`, {
        credentials: "include",
        headers: { "X-Requested-With": "hbmpanel" },
      }).then((r) => r.text());
      setBody(text);
    } finally {
      setBusy(false);
    }
  }

  async function runSearch(e: React.FormEvent) {
    e.preventDefault();
    if (!query.trim()) return;
    setBusy(true);
    try {
      const text = await fetch(
        `/api/logs/search?source=${encodeURIComponent(source)}&q=${encodeURIComponent(query)}&max=500`,
        { credentials: "include", headers: { "X-Requested-With": "hbmpanel" } }
      ).then((r) => r.text());
      setBody(text || "(no matches)");
    } finally {
      setBusy(false);
    }
  }

  function download() {
    const a = document.createElement("a");
    a.href = `/api/logs/download?source=${encodeURIComponent(source)}`;
    a.click();
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold">Logs</h1>
        <div className="flex flex-wrap items-center gap-2">
          <select
            className="rounded-xl bg-white/10 border border-white/15 px-3 py-1.5 text-sm"
            value={source}
            onChange={(e) => setSource(e.target.value)}
          >
            {(sources ?? []).map((s) => (
              <option key={s.key} value={s.key}>{s.label}</option>
            ))}
          </select>
          <div className="flex items-center gap-1 rounded-xl border border-white/15 bg-white/5 p-1 text-xs">
            <button
              onClick={() => setMode("live")}
              disabled={!liveTarget}
              className={
                "rounded-lg px-2 py-1 transition-colors " +
                (mode === "live" ? "bg-white/20 text-fg" : "text-muted hover:text-fg disabled:opacity-30")
              }
              title={liveTarget ? "Live tail via WS" : "Live tail tidak tersedia untuk source ini"}
            >
              <Radio size={12} className="inline" /> Live
            </button>
            <button
              onClick={() => setMode("search")}
              className={
                "rounded-lg px-2 py-1 transition-colors " +
                (mode === "search" ? "bg-white/20 text-fg" : "text-muted hover:text-fg")
              }
            >
              <Search size={12} className="inline" /> Search
            </button>
          </div>
          {mode === "search" ? (
            <Button variant="outline" disabled={busy} onClick={loadTail}>
              <RefreshCw size={14} /> Tail 500
            </Button>
          ) : null}
          <Button variant="outline" onClick={download}>
            <Download size={14} /> Download
          </Button>
        </div>
      </div>

      {mode === "search" && (
        <form onSubmit={runSearch} className="flex gap-2">
          <Input
            placeholder="cari teks dalam log…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <Button type="submit" disabled={busy}>
            <Search size={14} /> {busy ? "Searching…" : "Search"}
          </Button>
        </form>
      )}

      <Card className="p-0">
        <div
          ref={boxRef}
          className="scrollbar h-[70vh] overflow-auto font-mono text-xs p-3 leading-snug whitespace-pre"
        >
          {body || <span className="text-muted">
            {mode === "live"
              ? (liveTarget ? "connecting…" : "Source ini tidak mendukung live tail — pilih mode Search atau klik Tail 500.")
              : "Klik Tail 500 atau ketik query lalu Search."}
          </span>}
        </div>
      </Card>
    </div>
  );
}
