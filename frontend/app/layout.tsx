import "./globals.css";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "HBMPanel",
  description: "Lightweight modern app panel",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="dark">
      <body className="font-sans antialiased min-h-screen">{children}</body>
    </html>
  );
}
