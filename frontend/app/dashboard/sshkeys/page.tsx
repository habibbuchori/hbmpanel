"use client";
import { useState } from "react";
import useSWR, { mutate } from "swr";
import { Card, Button } from "@/components/ui";
import { api, fetcher } from "@/lib/api";
import { Trash2, Key } from "lucide-react";

type SSHKey = { type: string; comment: string; fingerprint: string };

export default function SSHKeysPage() {
  const { data: users } = useSWR<string[]>("/api/sshkeys/users", fetcher);
  const [user, setUser] = useState("root");
  const listKey = `/api/sshkeys/?user=${encodeURIComponent(user)}`;
  const { data: keys } = useSWR<SSHKey[]>(listKey, fetcher);

  const [newKey, setNewKey] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);

  async function addKey() {
    if (!newKey.trim()) return;
    setBusy(true);
    setErr(null);
    setMsg(null);
    try {
      await api("/api/sshkeys/", {
        method: "POST",
        body: JSON.stringify({ user, key: newKey.trim() }),
      });
      setNewKey("");
      setMsg("key ditambahkan");
      mutate(listKey);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "gagal");
    } finally {
      setBusy(false);
    }
  }

  async function remove(fp: string) {
    if (!confirm("Hapus public key ini? User dengan private key yang sesuai tidak akan bisa login lagi.")) return;
    await api(`/api/sshkeys/?user=${encodeURIComponent(user)}&fingerprint=${encodeURIComponent(fp)}`, {
      method: "DELETE",
    });
    mutate(listKey);
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">SSH keys</h1>
        <p className="text-sm text-muted">
          Kelola <code>~/.ssh/authorized_keys</code> per user. Pilih user di bawah lalu tambah atau hapus public key.
        </p>
      </div>

      <Card>
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">User</h2>
        <select
          value={user}
          onChange={(e) => setUser(e.target.value)}
          className="w-full rounded-xl bg-white/10 border border-white/15 px-3 py-2 text-sm font-mono"
        >
          {(users ?? ["root"]).map((u) => <option key={u} value={u}>{u}</option>)}
        </select>
      </Card>

      <Card>
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3 flex items-center gap-2">
          <Key size={14} /> Add public key
        </h2>
        <textarea
          value={newKey}
          onChange={(e) => setNewKey(e.target.value)}
          placeholder="ssh-ed25519 AAAA... user@host"
          className="mb-3 h-28 w-full rounded-xl bg-white/10 border border-white/15 px-3 py-2 font-mono text-xs"
        />
        <div className="flex items-center justify-between">
          {err ? <span className="text-xs text-red-300">{err}</span>
            : msg ? <span className="text-xs text-mint">{msg}</span>
            : <span />}
          <Button onClick={addKey} disabled={busy || !newKey.trim()}>
            {busy ? "Adding…" : "Add key"}
          </Button>
        </div>
      </Card>

      <Card>
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">
          Authorized keys for <span className="font-mono normal-case">{user}</span>
        </h2>
        <div className="divide-y divide-white/10">
          {(keys ?? []).map((k) => (
            <div key={k.fingerprint} className="flex items-center justify-between gap-3 py-3">
              <div className="min-w-0 space-y-0.5">
                <div className="font-mono text-xs">
                  <span className="text-brand">{k.type}</span> · {k.comment || <span className="text-muted">(no comment)</span>}
                </div>
                <div className="font-mono text-xs text-muted break-all">{k.fingerprint}</div>
              </div>
              <Button variant="danger" onClick={() => remove(k.fingerprint)}>
                <Trash2 size={14} /> Remove
              </Button>
            </div>
          ))}
          {keys && keys.length === 0 && (
            <div className="py-6 text-center text-sm text-muted">Belum ada key.</div>
          )}
        </div>
      </Card>
    </div>
  );
}
