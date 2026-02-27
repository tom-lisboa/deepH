/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  webpack: (config, { dev, isServer }) => {
    if (dev) {
      // Keep dev cache in memory only. This avoids flaky filesystem pack cache
      // failures (.pack.gz missing) seen in some local setups.
      config.cache = { type: "memory" };

      // Force server chunk emission under ./chunks so runtime resolves
      // require("./chunks/<id>.js") instead of require("./<id>.js").
      if (isServer && config.output) {
        config.output.chunkFilename = "chunks/[id].js";
      }
    }
    return config;
  }
};

export default nextConfig;
