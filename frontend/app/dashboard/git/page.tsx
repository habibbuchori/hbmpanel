"use client";
import { useState } from "react";
import useSWR, { mutate } from "swr";
import { Button, Card, Input } from "@/components/ui";
import { PremiumGate } from "@/components/premium";
import { api, fetcher } from "@/lib/api";
import { GitBranch, Rocket, Unlink } from "lucide-react";

type Site = { id: number; domain: string; root_path: string };

type Deployment = {
  site_id: number;
  repo: string;
  branch: string;
  deploy_cmd?: string;
  last_commit?: string;
  last_deployed_at?: string;
  last_output?: string;
  root_path?: string;
};

type StatusResp = { linked: boolean; root_path?: string; deployment?: Deployment };

export default function GitPage() {
  const { data: sites } = useSWR<Site[]>("/api/sites/", fetcher);
  const [siteId, setSiteId] = useState<number | null>(null);
  const selected = sites?.find((s) => s.id === siteId) ?? sites?.[0] ?? null;
  const id = selected?.id;

  const swrKey = id ? `/api/git/sites/${id}` : null;
  const { data: status, error } = useSWR<StatusResp>(swrKey, fetcher);

  if (error) return <PremiumGate error={error} />;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Git Deploy</h1>
        <p className="text-sm text-muted">
          Clone repo ke root_path site, lalu pull + run optional deploy command.
        </p>
      </div>

      <Card>
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">Site</h2>
        <select
          value={id ?? ""}
          onChange={(e) => setSiteId(Number(e.target.value))}
          className="w-full rounded-xl bg-white/10 border border-white/15 px-3 py-2 text-sm"
        >
          {(sites ?? []).map((s) => (
            <option key={s.id} value={s.id}>{s.domain} — {s.root_path || "(no root)"}</option>
          ))}
        </select>
      </Card>

      {!id ? null : !status ? (
        <Card><div className="text-sm text-muted">loading…</div></Card>
      ) : status.linked && status.deployment ? (
        <LinkedView siteId={id} d={status.deployment} />
      ) : (
        <SetupForm siteId={id} rootPath={status.root_path ?? ""} />
      )}
    </div>
  );
}

function SetupForm({ siteId, rootPath }: { siteId: number; rootPath: string }) {
  const [repo, setRepo] = useState("");
  const [branch, setBranch] = useState("main");
  const [deployCmd, setDeployCmd] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [output, setOutput] = useState("");

  async function setup(e: React.FormEvent) {
    e.preventDefault();
    if (!confirm(`Clone akan menghapus isi ${rootPath} dulu. Lanjut?`)) return;
    setBusy(true);
    setErr(null);
    setOutput("");
    try {
      const r = await api<{ commit: string; output: string }>(
        `/api/git/sites/${siteId}/setup`,
        { method: "POST", body: JSON.stringify({ repo, branch, deploy_cmd: deployCmd }) }
      );
      setOutput(r.output);
      mutate(`/api/git/sites/${siteId}`);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">
        <GitBranch size={14} className="inline mr-1" /> Setup git repo
      </h2>
      <form onSubmit={setup} className="space-y-3">
        <Input placeholder="https://github.com/user/repo.git or git@github.com:user/repo.git"
          value={repo} onChange={(e) => setRepo(e.target.value)} className="font-mono" />
        <div className="grid grid-cols-2 gap-3">
          <Input placeholder="branch" value={branch} onChange={(e) => setBranch(e.target.value)} />
          <Input placeholder="deploy command (opsional, mis: npm ci && npm run build)"
            value={deployCmd} onChange={(e) => setDeployCmd(e.target.value)} className="font-mono" />
        </div>
        <Button type="submit" disabled={busy || !repo}>
          {busy ? "Setting up…" : "Clone & link"}
        </Button>
        {err && <div className="text-xs text-red-300">{err}</div>}
        {output && (
          <pre className="scrollbar max-h-60 overflow-auto rounded-xl bg-black/30 p-3 font-mono text-xs whitespace-pre-wrap">
            {output}
          </pre>
        )}
      </form>
    </Card>
  );
}

function LinkedView({ siteId, d }: { siteId: number; d: Deployment }) {
  const [busy, setBusy] = useState(false);
  const [output, setOutput] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);

  async function deploy() {
    setBusy(true);
    setErr(null);
    try {
      const r = await api<{ output: string; last_commit: string }>(
        `/api/git/sites/${siteId}/deploy`, { method: "POST" }
      );
      setOutput(r.output);
      mutate(`/api/git/sites/${siteId}`);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(false);
    }
  }

  async function unlink() {
    if (!confirm("Unlink repo dari site? File di root_path tetap ada.")) return;
    await api(`/api/git/sites/${siteId}`, { method: "DELETE" });
    mutate(`/api/git/sites/${siteId}`);
  }

  return (
    <>
      <Card>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="space-y-1 text-sm">
            <div className="flex items-center gap-2">
              <GitBranch size={14} />
              <span className="font-mono">{d.repo}</span>
            </div>
            <div className="text-muted">branch: <span className="font-mono">{d.branch}</span></div>
            {d.deploy_cmd && (
              <div className="text-muted">deploy: <span className="font-mono">{d.deploy_cmd}</span></div>
            )}
            {d.last_commit && (
              <div className="text-muted">commit: <span className="font-mono">{d.last_commit.slice(0, 10)}</span></div>
            )}
            {d.last_deployed_at && (
              <div className="text-muted">last deploy: {new Date(d.last_deployed_at).toLocaleString()}</div>
            )}
          </div>
          <div className="flex items-center gap-2">
            <Button onClick={deploy} disabled={busy}>
              <Rocket size={14} /> {busy ? "Deploying…" : "Deploy"}
            </Button>
            <Button variant="outline" onClick={unlink}>
              <Unlink size={14} /> Unlink
            </Button>
          </div>
        </div>
        {err && <div className="mt-3 text-xs text-red-300">{err}</div>}
      </Card>

      {(output ?? d.last_output) && (
        <Card className="p-0">
          <div className="border-b border-white/10 px-4 py-3">
            <h2 className="text-sm font-semibold uppercase tracking-wider text-muted">
              {output ? "Latest output" : "Previous output"}
            </h2>
          </div>
          <pre className="scrollbar max-h-96 overflow-auto bg-black/30 p-4 font-mono text-xs whitespace-pre-wrap">
            {output ?? d.last_output}
          </pre>
        </Card>
      )}
    </>
  );
}
