"use client";
import { useEffect, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { Card } from "@/components/ui";

export default function TerminalPage() {
  const wrapRef = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState<"connecting" | "open" | "closed">("connecting");

  useEffect(() => {
    if (!wrapRef.current) return;

    const term = new Terminal({
      cursorBlink: true,
      fontFamily: '"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: 13,
      theme: {
        background: "#0f1028",
        foreground: "#f8fbff",
        cursor: "#7dd3fc",
        selectionBackground: "#7dd3fc55",
      },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(wrapRef.current);
    fit.fit();

    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${location.host}/api/terminal/shell`);

    function sendResize() {
      if (ws.readyState !== WebSocket.OPEN) return;
      ws.send(JSON.stringify({ resize: { cols: term.cols, rows: term.rows } }));
    }

    ws.onopen = () => {
      setStatus("open");
      sendResize();
    };
    ws.onmessage = (e) => term.write(typeof e.data === "string" ? e.data : "");
    ws.onclose = () => {
      setStatus("closed");
      term.writeln("\r\n\x1b[31m[connection closed]\x1b[0m");
    };
    ws.onerror = () => term.writeln("\r\n\x1b[31m[connection error]\x1b[0m");

    const dataSub = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ data }));
    });

    const resizeObs = new ResizeObserver(() => {
      try {
        fit.fit();
        sendResize();
      } catch {}
    });
    resizeObs.observe(wrapRef.current);

    return () => {
      dataSub.dispose();
      resizeObs.disconnect();
      ws.close();
      term.dispose();
    };
  }, []);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Terminal</h1>
        <span className={
          "text-xs " +
          (status === "open" ? "text-mint" : status === "closed" ? "text-red-300" : "text-muted")
        }>
          {status}
        </span>
      </div>
      <Card className="p-0">
        <div ref={wrapRef} className="h-[75vh] rounded-3xl bg-[#0f1028] p-3" />
      </Card>
      <p className="text-xs text-muted">
        Shell penuh ke server. Hati-hati — semua perintah dijalankan sebagai user panel (biasanya root).
      </p>
    </div>
  );
}
