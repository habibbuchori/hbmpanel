"use client";
import { useEffect, useRef, useState } from "react";
import { Card } from "@/components/ui";

const targets = ["panel", "caddy", "php", "postgres"] as const;
type Target = (typeof targets)[number];

export default function LogsPage() {
  const [target, setTarget] = useState<Target>("panel");
  const [lines, setLines] = useState<string[]>([]);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setLines([]);
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${location.host}/api/ws/logs/${target}`);
    ws.onmessage = (e) => {
      setLines((prev) => [...prev.slice(-2000), e.data as string]);
      requestAnimationFrame(() => {
        boxRef.current?.scrollTo(0, boxRef.current.scrollHeight);
      });
    };
    return () => ws.close();
  }, [target]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Logs</h1>
        <select className="rounded-md bg-panel border border-line px-3 py-1.5 text-sm"
          value={target} onChange={(e) => setTarget(e.target.value as Target)}>
          {targets.map((t) => <option key={t} value={t}>{t}</option>)}
        </select>
      </div>
      <Card className="p-0">
        <div ref={boxRef}
          className="scrollbar h-[70vh] overflow-auto font-mono text-xs p-3 leading-snug">
          {lines.length === 0 && <div className="text-muted">connecting…</div>}
          {lines.map((l, i) => <div key={i} className="whitespace-pre">{l}</div>)}
        </div>
      </Card>
    </div>
  );
}
