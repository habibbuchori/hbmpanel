"use client";
import { Card } from "@/components/ui";

export default function SettingsPage() {
  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Settings</h1>
      <Card>
        <p className="text-sm text-muted">
          Section ini akan berisi profile, ganti password, 2FA, token API, dan konfigurasi global panel.
          Belum di-scaffold pada MVP v0.1.
        </p>
      </Card>
    </div>
  );
}
