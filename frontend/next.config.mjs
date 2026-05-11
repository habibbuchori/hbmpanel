/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "export",
  images: { unoptimized: true },
  // trailingSlash sengaja di-OFF: bikin POST /api/* di-redirect 308 → GET dan
  // memecahkan login. SPA fallback ditangani oleh Go (NotFoundFile: index.html).
  reactStrictMode: true,
  async rewrites() {
    // Hanya aktif di dev (next dev). `next build` static export mengabaikan ini.
    return process.env.NODE_ENV === "development"
      ? [{ source: "/api/:path*", destination: "http://localhost:8443/api/:path*" }]
      : [];
  },
};

export default nextConfig;
