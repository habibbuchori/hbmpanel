import type { Config } from "tailwindcss";

export default {
  darkMode: "class",
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        bg:    "#0f1028",
        panel: "#181a3a",
        line:  "#2d315f",
        muted: "#a8b0d8",
        fg:    "#f8fbff",
        brand: "#7dd3fc",
        candy: "#f0abfc",
        sunny: "#fde68a",
        mint:  "#86efac",
      },
      fontFamily: {
        sans: ["ui-sans-serif", "system-ui", "Inter", "sans-serif"],
        mono: ["ui-monospace", "SFMono-Regular", "Menlo", "monospace"],
      },
    },
  },
  plugins: [],
} satisfies Config;
