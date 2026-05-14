"use client";
import Link from "next/link";
import { Card } from "@/components/ui";
import { Lock } from "lucide-react";

export function PremiumGate({ error }: { error: unknown }) {
  const msg = error instanceof Error ? error.message : String(error);
  const isPaywall = /premium|license|payment/i.test(msg);
  return (
    <Card>
      <div className="flex flex-col items-center gap-3 py-10 text-center">
        <div className="grid h-14 w-14 place-items-center rounded-2xl bg-gradient-to-br from-brand via-candy to-sunny shadow-lg shadow-candy/20">
          <Lock size={22} className="text-slate-950" />
        </div>
        <h2 className="text-lg font-bold">
          {isPaywall ? "Premium feature" : "Belum bisa load fitur ini"}
        </h2>
        <p className="max-w-md text-sm text-muted">
          {isPaywall
            ? "Fitur ini bagian dari premium tier v0.2+. Aktifkan license untuk membuka SFTP, File Manager, Backup, & Git Deploy."
            : msg}
        </p>
        {isPaywall && (
          <Link
            href="/dashboard/settings"
            className="inline-flex items-center gap-2 rounded-2xl bg-gradient-to-r from-brand via-candy to-sunny px-4 py-2 text-sm font-bold text-slate-950 shadow-lg shadow-candy/20"
          >
            Aktifkan license →
          </Link>
        )}
      </div>
    </Card>
  );
}
