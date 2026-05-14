"use client";
import { useState } from "react";
import useSWR from "swr";
import { Card, Button, Input } from "@/components/ui";
import { api, fetcher } from "@/lib/api";
import { Copy, Check, ShieldCheck, ShieldOff, KeyRound, Network } from "lucide-react";

type LicenseStatus = {
  status: string;
  plan: string;
  machine_id: string;
  features: string[];
  expires_at?: string;
  grace_until?: string;
};
type Me = { uid: number; role: string; totp_enabled: boolean };

const featureLabels: Record<string, string> = {
  sftp: "SFTP Management",
  multi_php: "Multi-PHP",
  file_manager: "File Manager",
  terminal: "Web Terminal",
  backup: "Backup & Restore",
  git_deploy: "Git Deployment",
};

export default function SettingsPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Settings</h1>
      <SecuritySection />
      <IPWhitelistSection />
      <LicenseSection />
    </div>
  );
}

// ============================================================
// SECURITY: change password + 2FA
// ============================================================
function SecuritySection() {
  const { data: me, mutate: refreshMe } = useSWR<Me>("/api/me", fetcher);
  const [setup, setSetup] = useState<{ secret: string; otpauth_url: string } | null>(null);

  async function start2FA() {
    const r = await api<{ secret: string; otpauth_url: string }>("/api/auth/2fa/setup", { method: "POST" });
    setSetup(r);
  }

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-bold flex items-center gap-2">
        <ShieldCheck size={18} /> Security
      </h2>

      <ChangePasswordCard />

      <Card>
        <div className="flex items-start justify-between gap-3">
          <div>
            <h3 className="text-sm font-bold uppercase tracking-wider text-muted mb-1">
              Two-factor authentication (TOTP)
            </h3>
            <p className="text-sm text-muted">
              {me?.totp_enabled
                ? "2FA aktif. Login butuh kode dari authenticator app."
                : "Lindungi akun dengan kode 6-digit dari Google Authenticator / Authy / 1Password."}
            </p>
          </div>
          {me?.totp_enabled ? (
            <DisableTOTPButton onDone={refreshMe} />
          ) : (
            <Button onClick={start2FA}>
              <ShieldCheck size={14} /> Enable 2FA
            </Button>
          )}
        </div>

        {setup && !me?.totp_enabled && (
          <EnableTOTPFlow
            secret={setup.secret}
            otpauthUrl={setup.otpauth_url}
            onCancel={() => setSetup(null)}
            onDone={() => { setSetup(null); refreshMe(); }}
          />
        )}
      </Card>
    </div>
  );
}

function ChangePasswordCard() {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setMsg(null);
    if (next !== confirm) {
      setMsg({ ok: false, text: "Konfirmasi tidak cocok" });
      return;
    }
    if (next.length < 10) {
      setMsg({ ok: false, text: "Password baru minimal 10 karakter" });
      return;
    }
    setBusy(true);
    try {
      await api("/api/auth/password", { method: "POST", body: JSON.stringify({ current, new: next }) });
      setCurrent(""); setNext(""); setConfirm("");
      setMsg({ ok: true, text: "Password berhasil diganti" });
    } catch (e) {
      setMsg({ ok: false, text: e instanceof Error ? e.message : "gagal" });
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      <h3 className="text-sm font-bold uppercase tracking-wider text-muted mb-3 flex items-center gap-2">
        <KeyRound size={14} /> Change password
      </h3>
      <form onSubmit={save} className="space-y-2">
        <Input type="password" placeholder="current password"
          value={current} onChange={(e) => setCurrent(e.target.value)} />
        <Input type="password" placeholder="new password (min 10 chars)"
          value={next} onChange={(e) => setNext(e.target.value)} />
        <Input type="password" placeholder="confirm new password"
          value={confirm} onChange={(e) => setConfirm(e.target.value)} />
        <div className="flex items-center justify-between">
          {msg ? (
            <span className={msg.ok ? "text-xs text-mint" : "text-xs text-red-300"}>{msg.text}</span>
          ) : <span />}
          <Button type="submit" disabled={busy || !current || !next || !confirm}>
            {busy ? "Saving…" : "Update password"}
          </Button>
        </div>
      </form>
    </Card>
  );
}

function EnableTOTPFlow({
  secret, otpauthUrl, onCancel, onDone,
}: { secret: string; otpauthUrl: string; onCancel: () => void; onDone: () => void }) {
  const qrSrc = `https://api.qrserver.com/v1/create-qr-code/?size=220x220&margin=0&data=${encodeURIComponent(otpauthUrl)}`;
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  function copySecret() {
    navigator.clipboard.writeText(secret);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      await api("/api/auth/2fa/enable", {
        method: "POST",
        body: JSON.stringify({ secret, code }),
      });
      onDone();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mt-4 grid gap-4 md:grid-cols-[220px_1fr] border-t border-white/10 pt-4">
      <div className="grid place-items-center">
        <img src={qrSrc} alt="2FA QR code" className="rounded-xl bg-white p-2" />
      </div>
      <div className="space-y-3">
        <p className="text-sm text-muted">
          1. Scan QR di authenticator app, <strong>atau</strong> input manual secret di bawah.<br />
          2. Masukkan kode 6-digit yang muncul untuk konfirmasi.
        </p>
        <div className="flex items-center gap-2 rounded-xl bg-black/30 px-3 py-2 font-mono text-xs">
          <span className="flex-1 break-all">{secret}</span>
          <button onClick={copySecret} className="text-muted hover:text-fg">
            {copied ? <Check size={14} /> : <Copy size={14} />}
          </button>
        </div>
        <form onSubmit={submit} className="flex items-center gap-2">
          <Input
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
            placeholder="6-digit code"
            inputMode="numeric"
            className="text-center font-mono tracking-[0.4em]"
          />
          <Button type="submit" disabled={busy || code.length !== 6}>
            {busy ? "Enabling…" : "Confirm"}
          </Button>
          <Button type="button" variant="ghost" onClick={onCancel}>Cancel</Button>
        </form>
        {err && <div className="text-xs text-red-300">{err}</div>}
      </div>
    </div>
  );
}

function DisableTOTPButton({ onDone }: { onDone: () => void }) {
  const [open, setOpen] = useState(false);
  const [pw, setPw] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      await api("/api/auth/2fa/disable", { method: "POST", body: JSON.stringify({ password: pw }) });
      setOpen(false);
      onDone();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(false);
    }
  }

  if (!open) {
    return (
      <Button variant="outline" onClick={() => setOpen(true)}>
        <ShieldOff size={14} /> Disable 2FA
      </Button>
    );
  }
  return (
    <form onSubmit={submit} className="flex items-center gap-2">
      <Input
        type="password"
        placeholder="confirm password"
        value={pw}
        onChange={(e) => setPw(e.target.value)}
        autoFocus
      />
      <Button type="submit" variant="danger" disabled={busy || !pw}>
        {busy ? "…" : "Disable"}
      </Button>
      <Button type="button" variant="ghost" onClick={() => setOpen(false)}>X</Button>
      {err && <span className="text-xs text-red-300">{err}</span>}
    </form>
  );
}

// ============================================================
// IP WHITELIST
// ============================================================
function IPWhitelistSection() {
  const { data, mutate: refresh } = useSWR<{ allowed_ips: string }>(
    "/api/settings/ip-whitelist/",
    fetcher
  );
  const [value, setValue] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);

  const current = value ?? data?.allowed_ips ?? "";

  async function save() {
    setBusy(true);
    setMsg(null);
    try {
      await api("/api/settings/ip-whitelist/", {
        method: "PUT",
        body: JSON.stringify({ allowed_ips: current }),
      });
      setValue(null);
      refresh();
      setMsg({ ok: true, text: "Whitelist tersimpan" });
    } catch (e) {
      setMsg({ ok: false, text: e instanceof Error ? e.message : "gagal" });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-3">
      <h2 className="text-lg font-bold flex items-center gap-2">
        <Network size={18} /> IP Whitelist
      </h2>
      <Card>
        <p className="text-sm text-muted mb-3">
          Daftar IP atau CIDR yang boleh akses <code>/api/*</code>. Kosongkan untuk allow-all.
          Pisahkan dengan koma, mis. <code>1.2.3.4, 10.0.0.0/8</code>. <strong>Hati-hati:</strong> kalau salah set,
          kamu kena lock-out — login lewat SSH lalu hapus row di <code>settings</code> table.
        </p>
        <textarea
          value={current}
          onChange={(e) => setValue(e.target.value)}
          placeholder="kosongkan untuk allow-all"
          className="mb-3 h-24 w-full rounded-xl bg-white/10 border border-white/15 px-3 py-2 font-mono text-xs"
        />
        <div className="flex items-center justify-between">
          {msg ? (
            <span className={msg.ok ? "text-xs text-mint" : "text-xs text-red-300"}>{msg.text}</span>
          ) : <span />}
          <Button onClick={save} disabled={busy || value === null}>
            {busy ? "Saving…" : "Save"}
          </Button>
        </div>
      </Card>
    </div>
  );
}

// ============================================================
// LICENSE
// ============================================================
function LicenseSection() {
  const { data: license, mutate: refreshLicense, isLoading } = useSWR<LicenseStatus>(
    "/api/license",
    fetcher,
    { refreshInterval: 30000 }
  );
  const [token, setToken] = useState("");
  const [activating, setActivating] = useState(false);
  const [error, setError] = useState("");
  const [refreshing, setRefreshing] = useState(false);

  async function handleActivate(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setActivating(true);
    try {
      await api("/api/license/activate", { method: "POST", body: JSON.stringify({ token }) });
      setToken("");
      refreshLicense();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to activate");
    } finally {
      setActivating(false);
    }
  }

  async function handleRefresh() {
    setRefreshing(true);
    try {
      await api("/api/license/refresh", { method: "POST" });
      refreshLicense();
    } finally {
      setRefreshing(false);
    }
  }

  return (
    <div className="space-y-3">
      <h2 className="text-lg font-bold">License</h2>
      <Card>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-bold uppercase tracking-wider text-muted">Status</h3>
            {license && (
              <span className={
                "rounded-full px-3 py-1 text-xs font-medium " +
                (license.status === "active" ? "bg-mint/20 text-mint"
                  : license.grace_until ? "bg-sunny/20 text-sunny"
                  : "bg-white/10 text-muted")
              }>
                {license.status === "active" ? "✓ Premium" : license.grace_until ? "⏱ Grace" : "○ Free"}
              </span>
            )}
          </div>
          {isLoading ? <p className="text-sm text-muted">Loading…</p> : license ? (
            <>
              <div>
                <label className="text-xs text-muted">Machine ID</label>
                <CopyableID value={license.machine_id} />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-xs text-muted">Plan</label>
                  <p className="text-sm font-medium">{license.plan}</p>
                </div>
                {license.expires_at && (
                  <div>
                    <label className="text-xs text-muted">Expires</label>
                    <p className="text-sm font-medium">{new Date(license.expires_at).toLocaleDateString()}</p>
                  </div>
                )}
              </div>
              {license.features?.length > 0 && (
                <div>
                  <label className="text-xs text-muted">Enabled features</label>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {license.features.map((f) => (
                      <span key={f} className="rounded-lg bg-mint/15 px-2 py-1 text-xs text-mint">
                        ✓ {featureLabels[f] || f}
                      </span>
                    ))}
                  </div>
                </div>
              )}
              {license.grace_until && (
                <p className="rounded-xl bg-sunny/10 px-3 py-2 text-xs text-sunny">
                  Grace period sampai {new Date(license.grace_until).toLocaleDateString()}. Offline OK.
                </p>
              )}
            </>
          ) : null}
        </div>
      </Card>

      <Card>
        <h3 className="text-sm font-bold uppercase tracking-wider text-muted mb-3">Activate license</h3>
        <form onSubmit={handleActivate} className="space-y-2">
          <Input placeholder="HBM-XXXX-XXXX-XXXX"
            value={token} onChange={(e) => setToken(e.target.value)} disabled={activating} />
          {error && <p className="text-xs text-red-300">{error}</p>}
          <Button disabled={activating || !token.trim()} className="w-full">
            {activating ? "Validating…" : "Activate"}
          </Button>
        </form>
      </Card>

      {license && license.status === "active" && (
        <Card>
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-bold">Re-validate</h3>
              <p className="text-xs text-muted">Cek status license dengan server</p>
            </div>
            <Button onClick={handleRefresh} disabled={refreshing} variant="outline">
              {refreshing ? "Refreshing…" : "Re-validate"}
            </Button>
          </div>
        </Card>
      )}
    </div>
  );
}

function CopyableID({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="flex items-center gap-2 rounded-xl bg-white/10 px-3 py-2 font-mono text-xs">
      <span className="flex-1 break-all">{value}</span>
      <button
        onClick={() => {
          navigator.clipboard.writeText(value);
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        }}
        className="text-muted hover:text-fg"
      >
        {copied ? <Check size={14} /> : <Copy size={14} />}
      </button>
    </div>
  );
}
