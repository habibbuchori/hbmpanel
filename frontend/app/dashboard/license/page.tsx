"use client";
import { useState } from "react";
import useSWR from "swr";
import { Card, Button, Input } from "@/components/ui";
import { api, fetcher } from "@/lib/api";
import { Ticket, RefreshCw, Copy, Check } from "lucide-react";

type Me = { uid: number; role: string; totp_enabled: boolean };

const FEATURES = [
  { id: "sftp",         label: "SFTP Management" },
  { id: "file_manager", label: "File Manager" },
  { id: "backup",       label: "Backup & Restore" },
  { id: "git_deploy",   label: "Git Deployment" },
] as const;

function makeSeg(): string {
  return Math.floor(Math.random() * 0xffff).toString(36).toUpperCase().padStart(4, "0").slice(-4);
}
function generateToken(): string {
  return `HBM-${makeSeg()}-${makeSeg()}-${makeSeg()}`;
}

export default function LicensePage() {
  const { data: me } = useSWR<Me>("/api/me", fetcher);

  const [token,     setToken]     = useState(() => generateToken());
  const [plan,      setPlan]      = useState("");
  const [features,  setFeatures]  = useState<string[]>([]);
  const [expiresAt, setExpiresAt] = useState("");
  const [busy,      setBusy]      = useState(false);
  const [error,     setError]     = useState<string | null>(null);
  const [result,    setResult]    = useState<string | null>(null);
  const [copied,    setCopied]    = useState(false);

  if (me && me.role !== "admin") {
    return (
      <Card>
        <p className="text-sm text-muted py-8 text-center">Access denied — admin only.</p>
      </Card>
    );
  }

  function toggleFeature(id: string) {
    setFeatures(prev => prev.includes(id) ? prev.filter(f => f !== id) : [...prev, id]);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await api("/api/admin/license/token", {
        method: "POST",
        body: JSON.stringify({ token, plan, features, expires_at: expiresAt || undefined }),
      });
      setResult(token);
    } catch (e) {
      setError(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(false);
    }
  }

  function reset() {
    setResult(null);
    setToken(generateToken());
    setPlan("");
    setFeatures([]);
    setExpiresAt("");
    setError(null);
  }

  if (result) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-semibold">Generate License Token</h1>
        <Card>
          <h2 className="text-sm font-bold uppercase tracking-wider text-muted mb-3 flex items-center gap-2">
            <Ticket size={14} /> Token berhasil dibuat
          </h2>
          <div className="flex items-center gap-2 rounded-xl bg-black/30 px-3 py-2 font-mono text-sm">
            <span className="flex-1 break-all">{result}</span>
            <button
              onClick={() => {
                navigator.clipboard.writeText(result);
                setCopied(true);
                setTimeout(() => setCopied(false), 1500);
              }}
              className="text-muted hover:text-fg"
            >
              {copied ? <Check size={14} /> : <Copy size={14} />}
            </button>
          </div>
          <div className="mt-4">
            <Button onClick={reset}>Buat lagi</Button>
          </div>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Generate License Token</h1>
      <Card>
        <form onSubmit={submit} className="space-y-5">
          <div>
            <label className="text-xs text-muted uppercase tracking-wider block mb-1">Token</label>
            <div className="flex gap-2">
              <Input
                value={token}
                onChange={e => setToken(e.target.value)}
                className="font-mono"
                placeholder="HBM-XXXX-XXXX-XXXX"
              />
              <Button type="button" variant="outline" onClick={() => setToken(generateToken())}>
                <RefreshCw size={14} />
              </Button>
            </div>
          </div>

          <div>
            <label className="text-xs text-muted uppercase tracking-wider block mb-1">Plan</label>
            <Input
              value={plan}
              onChange={e => setPlan(e.target.value)}
              placeholder="e.g. pro, enterprise"
            />
          </div>

          <div>
            <label className="text-xs text-muted uppercase tracking-wider block mb-2">Features</label>
            <div className="flex flex-wrap gap-4">
              {FEATURES.map(({ id, label }) => (
                <label key={id} className="flex items-center gap-2 text-sm cursor-pointer select-none">
                  <input
                    type="checkbox"
                    checked={features.includes(id)}
                    onChange={() => toggleFeature(id)}
                    className="rounded"
                  />
                  {label}
                </label>
              ))}
            </div>
          </div>

          <div>
            <label className="text-xs text-muted uppercase tracking-wider block mb-1">
              Expires at <span className="normal-case">(opsional)</span>
            </label>
            <Input
              type="date"
              value={expiresAt}
              onChange={e => setExpiresAt(e.target.value)}
              className="w-52"
            />
          </div>

          {error && <p className="text-xs text-red-300">{error}</p>}

          <Button type="submit" disabled={busy || !token.trim() || !plan.trim()}>
            {busy ? "Membuat…" : "Buat token"}
          </Button>
        </form>
      </Card>
    </div>
  );
}
