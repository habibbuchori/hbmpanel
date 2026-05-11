"use client";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";
import {
  LayoutDashboard, Globe, Server, Database, FileText, Settings, LogOut,
} from "lucide-react";
import { api } from "@/lib/api";
import { cn } from "@/lib/cn";

const nav = [
  { href: "/dashboard",          label: "Overview", icon: LayoutDashboard },
  { href: "/dashboard/sites",    label: "Sites",    icon: Globe },
  { href: "/dashboard/services", label: "Services", icon: Server },
  { href: "/dashboard/database", label: "Database", icon: Database },
  { href: "/dashboard/logs",     label: "Logs",     icon: FileText },
  { href: "/dashboard/settings", label: "Settings", icon: Settings },
];

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    api("/api/me").catch(() => router.replace("/"));
  }, [router]);

  async function logout() {
    await api("/api/auth/logout", { method: "POST" }).catch(() => {});
    router.replace("/");
  }

  return (
    <div className="min-h-screen flex">
      <aside className="w-56 border-r border-line bg-panel flex flex-col">
        <div className="p-4 border-b border-line">
          <div className="font-bold tracking-tight">HBMPanel</div>
          <div className="text-xs text-muted">v0.1.0-dev</div>
        </div>
        <nav className="flex-1 p-2 space-y-1">
          {nav.map(({ href, label, icon: Icon }) => {
            const active = pathname === href || (href !== "/dashboard" && pathname?.startsWith(href));
            return (
              <Link
                key={href}
                href={href}
                className={cn(
                  "flex items-center gap-2 px-3 py-2 rounded-md text-sm transition-colors",
                  active ? "bg-line text-fg" : "text-muted hover:text-fg hover:bg-line/40"
                )}
              >
                <Icon size={16} /> {label}
              </Link>
            );
          })}
        </nav>
        <button
          onClick={logout}
          className="m-2 flex items-center gap-2 px-3 py-2 rounded-md text-sm text-muted hover:text-fg hover:bg-line/40 transition-colors"
        >
          <LogOut size={16} /> Logout
        </button>
      </aside>
      <main className="flex-1 p-6 scrollbar overflow-auto">{children}</main>
    </div>
  );
}
