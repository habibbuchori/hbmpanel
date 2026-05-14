"use client";
import * as React from "react";
import { cn } from "@/lib/cn";

export function Button({
  className,
  variant = "default",
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "default" | "outline" | "ghost" | "danger";
}) {
  const variants = {
    default: "bg-gradient-to-r from-brand via-candy to-sunny text-slate-950 shadow-lg shadow-candy/20 hover:scale-[1.02]",
    outline: "border border-white/15 bg-white/5 hover:bg-white/10",
    ghost:   "hover:bg-white/10",
    danger:  "bg-red-500 hover:bg-red-400 text-white shadow-lg shadow-red-500/20",
  };
  return (
    <button
      className={cn(
        "inline-flex items-center justify-center gap-2 rounded-xl px-3.5 py-2 text-sm font-bold transition-all disabled:opacity-50 disabled:pointer-events-none",
        variants[variant],
        className
      )}
      {...props}
    />
  );
}

export function Input(props: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      {...props}
      className={cn(
        "w-full rounded-xl bg-white/10 border border-white/15 px-3 py-2 text-sm outline-none focus:border-brand focus:ring-4 focus:ring-brand/15 transition-all placeholder:text-muted",
        props.className
      )}
    />
  );
}

export function Card({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("rounded-3xl border border-white/12 bg-white/10 p-4 shadow-xl shadow-black/10 backdrop-blur-xl", className)}
      {...props}
    />
  );
}

export function StatTile({
  label, value, hint,
}: {
  label: string; value: React.ReactNode; hint?: string;
}) {
  return (
    <Card className="relative overflow-hidden">
      <div className="absolute -right-6 -top-6 h-20 w-20 rounded-full bg-brand/20 blur-2xl" />
      <div className="text-xs uppercase tracking-wider text-muted">{label}</div>
      <div className="mt-1 text-2xl font-semibold">{value}</div>
      {hint && <div className="mt-1 text-xs text-muted">{hint}</div>}
    </Card>
  );
}
