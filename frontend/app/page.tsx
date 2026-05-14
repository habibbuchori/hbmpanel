"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Input, Card } from "@/components/ui";
import { api } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [step, setStep] = useState<"password" | "totp">("password");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    api("/api/me").then(() => router.replace("/dashboard")).catch(() => {});
  }, [router]);

  async function submitPassword(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const r = await api<{ ok?: boolean; require_2fa?: boolean }>(
        "/api/auth/login",
        { method: "POST", body: JSON.stringify({ username, password }) }
      );
      if (r.require_2fa) {
        setStep("totp");
      } else {
        router.replace("/dashboard");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login gagal");
    } finally {
      setLoading(false);
    }
  }

  async function submitTOTP(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await api("/api/auth/2fa/verify", {
        method: "POST",
        body: JSON.stringify({ code }),
      });
      router.replace("/dashboard");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Kode 2FA tidak valid");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="relative min-h-screen grid place-items-center overflow-hidden px-4">
      <div className="absolute left-10 top-10 text-6xl opacity-30">✨</div>
      <div className="absolute bottom-12 right-12 text-7xl opacity-25">🚀</div>
      <Card className="w-full max-w-sm border-white/20 bg-white/12 p-6">
        <div className="mb-6 text-center">
          <div className="mx-auto mb-3 grid h-14 w-14 place-items-center rounded-2xl bg-gradient-to-br from-brand via-candy to-sunny text-2xl shadow-lg shadow-candy/20">⚡</div>
          <div className="text-2xl font-black tracking-tight">HBMPanel</div>
          <div className="text-sm text-muted">Fresh control room buat app kamu</div>
        </div>

        {step === "password" ? (
          <form onSubmit={submitPassword} className="space-y-3">
            <div>
              <label className="text-xs font-bold uppercase tracking-wide text-muted">Username</label>
              <Input value={username} onChange={(e) => setUsername(e.target.value)} autoFocus />
            </div>
            <div>
              <label className="text-xs font-bold uppercase tracking-wide text-muted">Password</label>
              <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
            </div>
            {error && (
              <div className="text-xs text-red-100 bg-red-500/20 border border-red-300/20 rounded-xl p-2">
                {error}
              </div>
            )}
            <Button type="submit" disabled={loading} className="w-full">
              {loading ? "Signing in…" : "Sign in"}
            </Button>
          </form>
        ) : (
          <form onSubmit={submitTOTP} className="space-y-3">
            <div>
              <label className="text-xs font-bold uppercase tracking-wide text-muted">
                6-digit code from authenticator
              </label>
              <Input
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                autoFocus
                inputMode="numeric"
                className="text-center text-2xl tracking-[0.4em] font-mono"
                placeholder="••••••"
              />
            </div>
            {error && (
              <div className="text-xs text-red-100 bg-red-500/20 border border-red-300/20 rounded-xl p-2">
                {error}
              </div>
            )}
            <Button type="submit" disabled={loading || code.length !== 6} className="w-full">
              {loading ? "Verifying…" : "Verify"}
            </Button>
            <button
              type="button"
              onClick={() => { setStep("password"); setCode(""); setError(null); }}
              className="w-full text-xs text-muted hover:text-fg"
            >
              ← back
            </button>
          </form>
        )}
      </Card>
    </main>
  );
}
