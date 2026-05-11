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
    default: "bg-brand text-black hover:bg-cyan-300",
    outline: "border border-line hover:bg-panel",
    ghost:   "hover:bg-panel",
    danger:  "bg-red-600 hover:bg-red-500 text-white",
  };
  return (
    <button
      className={cn(
        "inline-flex items-center justify-center gap-2 rounded-md px-3 py-1.5 text-sm font-medium transition-colors disabled:opacity-50 disabled:pointer-events-none",
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
        "w-full rounded-md bg-panel border border-line px-3 py-2 text-sm outline-none focus:border-brand transition-colors",
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
      className={cn("rounded-lg border border-line bg-panel p-4", className)}
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
    <Card>
      <div className="text-xs uppercase tracking-wider text-muted">{label}</div>
      <div className="mt-1 text-2xl font-semibold">{value}</div>
      {hint && <div className="mt-1 text-xs text-muted">{hint}</div>}
    </Card>
  );
}
