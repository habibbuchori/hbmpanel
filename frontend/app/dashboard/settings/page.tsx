"use client";
import { Card, Button } from "@/components/ui";
import { api, fetcher } from "@/lib/api";
import useSWR from "swr";
import { useState } from "react";
import { Copy, Check } from "lucide-react";

type LicenseStatus = {
  status: string;
  plan: string;
  machine_id: string;
  features: string[];
  expires_at?: string;
  grace_until?: string;
};

const featureLabels: Record<string, string> = {
  sftp: "SFTP Management",
  multi_php: "Multi-PHP",
  file_manager: "File Manager",
  terminal: "Web Terminal",
  backup: "Backup & Restore",
  git_deploy: "Git Deployment",
};

function StatusBadge({ status, graceUntil }: { status: string; graceUntil?: string }) {
  if (status === "active") {
    return (
      <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-green-400/10 text-green-400">
        ✓ Premium Active
      </span>
    );
  }
  if (graceUntil) {
    return (
      <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-amber-400/10 text-amber-400">
        ⏱ Grace Period
      </span>
    );
  }
  return (
    <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-muted text-muted-foreground">
      ○ Free / Inactive
    </span>
  );
}

function CopyableID({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = () => {
    navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };
  return (
    <div className="flex items-center gap-2 bg-muted px-3 py-2 rounded font-mono text-xs">
      <span className="flex-1 break-all">{value}</span>
      <button onClick={handleCopy} className="text-muted-foreground hover:text-foreground">
        {copied ? <Check size={16} /> : <Copy size={16} />}
      </button>
    </div>
  );
}

export default function SettingsPage() {
  const { data: license, mutate: mutateStatus, isLoading } = useSWR<LicenseStatus>(
    "/api/license",
    fetcher,
    { refreshInterval: 30000 }
  );
  const [token, setToken] = useState("");
  const [activating, setActivating] = useState(false);
  const [error, setError] = useState("");
  const [refreshing, setRefreshing] = useState(false);

  const handleActivate = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setActivating(true);
    try {
      const res = await api("/api/license/activate", {
        method: "POST",
        body: JSON.stringify({ token }),
      });
      setToken("");
      mutateStatus();
    } catch (err: any) {
      setError(err.message || "Failed to activate license");
    } finally {
      setActivating(false);
    }
  };

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      await api("/api/license/refresh", { method: "POST" });
      mutateStatus();
    } catch (err) {
      console.error("Refresh failed:", err);
    } finally {
      setRefreshing(false);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Settings</h1>
      </div>

      {/* License Status Card */}
      <Card>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold">License Status</h2>
            {license && <StatusBadge status={license.status} graceUntil={license.grace_until} />}
          </div>

          {isLoading ? (
            <p className="text-sm text-muted">Loading...</p>
          ) : license ? (
            <div className="space-y-3">
              <div>
                <label className="text-xs font-medium text-muted-foreground">Machine ID</label>
                <CopyableID value={license.machine_id} />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-xs font-medium text-muted-foreground">Plan</label>
                  <p className="text-sm font-medium mt-1">{license.plan}</p>
                </div>
                {license.expires_at && (
                  <div>
                    <label className="text-xs font-medium text-muted-foreground">Expires</label>
                    <p className="text-sm font-medium mt-1">
                      {new Date(license.expires_at).toLocaleDateString()}
                    </p>
                  </div>
                )}
              </div>

              {license.features && license.features.length > 0 && (
                <div>
                  <label className="text-xs font-medium text-muted-foreground">Enabled Features</label>
                  <div className="flex flex-wrap gap-2 mt-2">
                    {license.features.map((feat) => (
                      <span
                        key={feat}
                        className="inline-flex items-center px-2 py-1 rounded text-xs bg-green-400/10 text-green-400"
                      >
                        ✓ {featureLabels[feat] || feat}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {license.grace_until && (
                <p className="text-xs text-amber-600 bg-amber-400/10 px-3 py-2 rounded">
                  Grace period active until {new Date(license.grace_until).toLocaleDateString()}. Offline access allowed.
                </p>
              )}
            </div>
          ) : null}
        </div>
      </Card>

      {/* Activate License Form */}
      <Card>
        <div className="space-y-4">
          <h2 className="text-lg font-semibold">Activate License</h2>
          <form onSubmit={handleActivate} className="space-y-3">
            <div>
              <label className="text-xs font-medium text-muted-foreground">Token</label>
              <input
                type="text"
                placeholder="HBM-XXXX-XXXX-XXXX"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                disabled={activating}
                className="w-full mt-1 px-3 py-2 rounded border border-border bg-background text-foreground text-sm placeholder:text-muted-foreground disabled:opacity-50"
              />
            </div>
            {error && <p className="text-xs text-red-400">{error}</p>}
            <Button disabled={activating || !token.trim()} className="w-full">
              {activating ? "Validating..." : "Activate License"}
            </Button>
          </form>
        </div>
      </Card>

      {/* Refresh Button */}
      {license && license.status === "active" && (
        <Card>
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-lg font-semibold">Re-validate</h2>
              <p className="text-xs text-muted-foreground mt-1">
                Manually check license status with server
              </p>
            </div>
            <Button onClick={handleRefresh} disabled={refreshing} variant="outline">
              {refreshing ? "Refreshing..." : "Re-validate Now"}
            </Button>
          </div>
        </Card>
      )}
    </div>
  );
}
