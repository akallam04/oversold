import { loadEnv, defineConfig } from "@medusajs/framework/utils"

loadEnv(process.env.NODE_ENV || "production", process.cwd())

const locking = process.env.OVERSOLD_LOCKING || "in-memory"
if (locking !== "in-memory" && locking !== "redis") {
  throw new Error(`OVERSOLD_LOCKING is "${locking}", want in-memory or redis`)
}

module.exports = defineConfig({
  projectConfig: {
    databaseUrl: process.env.DATABASE_URL,
    http: {
      storeCors: process.env.STORE_CORS!,
      adminCors: process.env.ADMIN_CORS!,
      authCors: process.env.AUTH_CORS!,
      jwtSecret: process.env.JWT_SECRET!,
      cookieSecret: process.env.COOKIE_SECRET!,
    },
  },
  admin: {
    disable: true,
  },
  ...(locking === "redis"
    ? {
        modules: [
          {
            resolve: "@medusajs/medusa/locking",
            options: {
              providers: [
                {
                  resolve: "@medusajs/medusa/locking-redis",
                  id: "locking-redis",
                  is_default: true,
                  options: {
                    redisUrl: process.env.LOCKING_REDIS_URL,
                  },
                },
              ],
            },
          },
        ],
      }
    : {}),
})
