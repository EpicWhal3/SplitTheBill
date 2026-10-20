import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  allowedDevOrigins: ["192.168.56.1"],
  output: "standalone",
  rewrites: async () => {
    const backendURL = process.env.BACKEND_URL || "http://localhost:8080";

    return [
      {
        source: "/api/:path*",
        destination: `${backendURL}/:path*`,
      },
    ];
  },
};

export default nextConfig;
