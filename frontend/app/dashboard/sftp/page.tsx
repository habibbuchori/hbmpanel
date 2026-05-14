"use client";
import { useState } from "react";
import useSWR, { mutate } from "swr";
import { Button, Card, Input } from "@/components/ui";
import { PremiumGate } from "@/components/premium";
import { api, fetcher } from "@/lib/api";
import { KeyRound, Trash2 } from "lucide-react";

type SftpUser = { id: number; username: string; site_id?: number; home: string; created_at: string };
type Site = { id: number; domain: string };

export default function SftpPage() {
  const { data: users, error } = useSWR<SftpUser[]>("/api/sftp/users", fetcher);
  const { data: sites } = useSWR<Site[]>("/api/sites/", fetcher);

  const [form, setForm] = useState({ username: "", password: "", site_id: 0 });
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [pwUser, setPwUser] = useState<SftpUser | null>(null);

  if (error) return <PremiumGate error={error} />;

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      await api("/api/sftp/users", { method: "POST", body: JSON.stringify(form) });
      setForm({ username: "", password: "", site_id: 0 });
      mutate("/api/sftp/users");
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: number) {
    if (!confirm("Hapus SFTP user ini? (file di home dir juga dihapus)")) return;
    await api(`/api/sftp/users/${id}`, { method: "DELETE" });
    mutate("/api/sftp/users");
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">SFTP Users</h1>
        <p className="text-sm text-muted">
          Chroot-SFTP user. Login pakai SSH ke port 22, user dipenjara di <code>/home/&lt;user&gt;</code>.
          Write area di <code>/home/&lt;user&gt;/data</code>.
        </p>
      </div>

      <Card>
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">Create user</h2>
        <form onSubmit={create} className="grid grid-cols-1 md:grid-cols-4 gap-3">
          <Input
            placeholder="username (lowercase)"
            value={form.username}
            onChange={(e) => setForm({ ...form, username: e.target.value })}
          />
          <Input
            type="password"
            placeholder="password (min 8)"
            value={form.password}
            onChange={(e) => setForm({ ...form, password: e.target.value })}
          />
          <select
            value={form.site_id || ""}
            onChange={(e) => setForm({ ...form, site_id: Number(e.target.value) })}
            className="rounded-xl bg-white/10 border border-white/15 px-3 py-2 text-sm"
          >
            <option value="">no site link</option>
            {(sites ?? []).map((s) => (
              <option key={s.id} value={s.id}>{s.domain}</option>
            ))}
          </select>
          <Button type="submit" disabled={busy}>{busy ? "Creating…" : "Create"}</Button>
        </form>
        {err && <div className="mt-2 text-xs text-red-300">{err}</div>}
      </Card>

      <Card>
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">All users</h2>
        <div className="divide-y divide-white/10">
          {(users ?? []).map((u) => (
            <div key={u.id} className="flex items-center justify-between py-3">
              <div>
                <div className="font-mono">{u.username}</div>
                <div className="text-xs text-muted">{u.home}</div>
              </div>
              <div className="flex items-center gap-2">
                <Button variant="outline" onClick={() => setPwUser(u)}>
                  <KeyRound size={14} /> Password
                </Button>
                <Button variant="danger" onClick={() => remove(u.id)}>
                  <Trash2 size={14} /> Delete
                </Button>
              </div>
            </div>
          ))}
          {users && users.length === 0 && (
            <div className="py-6 text-center text-sm text-muted">Belum ada user.</div>
          )}
        </div>
      </Card>

      {pwUser && <PasswordModal user={pwUser} onClose={() => setPwUser(null)} />}
    </div>
  );
}

function PasswordModal({ user, onClose }: { user: SftpUser; onClose: () => void }) {
  const [pw, setPw] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      await api(`/api/sftp/users/${user.id}/password`, {
        method: "POST",
        body: JSON.stringify({ password: pw }),
      });
      onClose();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 px-4 backdrop-blur-sm" onClick={onClose}>
      <form
        onSubmit={save}
        className="w-full max-w-md space-y-3 rounded-3xl border border-white/15 bg-panel p-5 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <h3 className="text-sm font-bold uppercase tracking-wider text-muted">
          Reset password · {user.username}
        </h3>
        <Input
          type="password"
          placeholder="new password (min 8)"
          value={pw}
          onChange={(e) => setPw(e.target.value)}
          autoFocus
        />
        {err && <div className="text-xs text-red-300">{err}</div>}
        <div className="flex justify-end gap-2">
          <Button variant="ghost" type="button" onClick={onClose}>Cancel</Button>
          <Button type="submit" disabled={busy}>{busy ? "Saving…" : "Save"}</Button>
        </div>
      </form>
    </div>
  );
}
