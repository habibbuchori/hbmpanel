"use client";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";
import {
  LayoutDashboard, Globe, Server, Database, FileText, Settings, LogOut, Sparkles,
  Terminal, Users, FolderTree, Archive, GitBranch, ShieldCheck, Key,
} from "lucide-react";
import { api } from "@/lib/api";
import { cn } from "@/lib/cn";

const nav = [
  { href: "/dashboard",          label: "Overview",   icon: LayoutDashboard, premium: false },
  { href: "/dashboard/sites",    label: "Sites",      icon: Globe,           premium: false },
  { href: "/dashboard/laravel",  label: "Laravel",    icon: Sparkles,        premium: false },
  { href: "/dashboard/services", label: "Services",   icon: Server,          premium: false },
  { href: "/dashboard/database", label: "Database",   icon: Database,        premium: false },
  { href: "/dashboard/logs",     label: "Logs",       icon: FileText,        premium: false },
  { href: "/dashboard/terminal", label: "Terminal",   icon: Terminal,        premium: false },
  { href: "/dashboard/sshkeys",  label: "SSH keys",   icon: Key,             premium: false },
  { href: "/dashboard/audit",    label: "Audit log",  icon: ShieldCheck,     premium: false },
  { href: "/dashboard/files",    label: "Files",      icon: FolderTree,      premium: true  },
  { href: "/dashboard/sftp",     label: "SFTP",       icon: Users,           premium: true  },
  { href: "/dashboard/backups",  label: "Backups",    icon: Archive,         premium: true  },
  { href: "/dashboard/git",      label: "Git Deploy", icon: GitBranch,       premium: true  },
  { href: "/dashboard/settings", label: "Settings",   icon: Settings,        premium: false },
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
      <aside className="w-60 border-r border-white/10 bg-white/10 flex flex-col backdrop-blur-xl">
        <div className="p-4 border-b border-white/10">
          <div className="flex items-center gap-3">
            <div className="grid h-10 w-10 place-items-center rounded-2xl bg-gradient-to-br from-brand via-candy to-sunny text-lg shadow-lg shadow-candy/20">⚡</div>
            <div>
              <div className="font-black tracking-tight">HBMPanel</div>
              <div className="text-xs text-muted">v0.1.0-dev · fun mode</div>
            </div>
          </div>
        </div>
        <nav className="flex-1 p-2 space-y-1 overflow-y-auto scrollbar">
          {nav.map(({ href, label, icon: Icon, premium }) => {
            const active = pathname === href || (href !== "/dashboard" && pathname?.startsWith(href));
            return (
              <Link
                key={href}
                href={href}
                className={cn(
                  "flex items-center gap-2 px-3 py-2 rounded-2xl text-sm font-semibold transition-all",
                  active ? "bg-gradient-to-r from-brand/25 to-candy/25 text-fg shadow-lg shadow-candy/10" : "text-muted hover:text-fg hover:bg-white/10"
                )}
              >
                <Icon size={16} />
                <span className="flex-1">{label}</span>
                {premium && (
                  <span className="rounded-full bg-gradient-to-r from-brand/30 to-candy/30 px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wide text-fg">
                    pro
                  </span>
                )}
              </Link>
            );
          })}
        </nav>
        <button
          onClick={logout}
          className="m-2 flex items-center gap-2 px-3 py-2 rounded-2xl text-sm font-semibold text-muted hover:text-fg hover:bg-white/10 transition-all"
        >
          <LogOut size={16} /> Logout
        </button>
      </aside>
      <main className="flex-1 p-6 scrollbar overflow-auto">{children}</main>
    </div>
  );
}
