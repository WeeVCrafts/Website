import { defineRailway, github, preserve, project, service } from "railway/iac";

// This repository manages only its own resources in the environment. Other
// repositories export their own partial name.
// See https://docs.railway.com/infrastructure-as-code#multi-repo-projects
export const partial = "Website";

export default defineRailway(() => {
  const Website = service("Website", {
    source: github("WeeVCrafts/Website", { branch: "main" }),
    healthcheck: "/health/ready",
    healthcheckTimeout: 300,
    variables: {
      DATABASE_URL: "${{Postgres.DATABASE_URL}}?sslmode=verify-full",
      DATABASE_CA_CERT: preserve(),
      REDIS_URL: "${{Redis.REDIS_URL}}",
      S3_ADDR: "${{weevcrafts-media.ENDPOINT}}",
      S3_ACCESS_KEY: preserve(),
      S3_SECRET_KEY: preserve(),
      S3_BUCKET: "${{weevcrafts-media.BUCKET}}",
      S3_SECURE: "true",
      WECRATFS_ENV: "production",
      WECRATFS_SECURE_COOKIES: "true",
      WECRATFS_PUBLIC_URL: preserve(),
      WECRATFS_LOCAL_MODE: "false",
      WECRATFS_PAYMENT_MODE: "razorpay",
      RAZORPAY_KEY_ID: preserve(),
      RAZORPAY_KEY_SECRET: preserve(),
      RAZORPAY_WEBHOOK_SECRET: preserve(),
      WECRATFS_LOCAL_PAYMENT_SECRET: preserve(),
    },
  });
  return project("weevcrafts", {
    resources: [Website],
  });
});
