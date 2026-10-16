/**
 * Operator-side bridge: shell out to infra/aws `make watch` when present.
 *
 * Env:
 *   CAMPAIGN_ID — forwarded to make (required by the Makefile target)
 *
 * Live viz UI: open http://localhost:5173/?live=1 (or use the Live toggle).
 * The browser never talks to S3; this laptop process keeps the bucket private.
 */
import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "../../..");
const awsDir = path.join(repoRoot, "infra", "aws");
const makefile = path.join(awsDir, "Makefile");

const campaignId = process.env.CAMPAIGN_ID || "";

function runMakeWatch() {
  const args = ["watch"];
  if (campaignId) {
    args.push(`CAMPAIGN_ID=${campaignId}`);
  }
  console.log(`[watch-s3] running: make ${args.join(" ")} (cwd=${awsDir})`);
  const child = spawn("make", args, {
    cwd: awsDir,
    stdio: "inherit",
    shell: true,
    env: process.env,
  });
  child.on("exit", (code, signal) => {
    if (signal) {
      process.kill(process.pid, signal);
      return;
    }
    process.exit(code ?? 1);
  });
}

function printFallback() {
  console.error(`
[watch-s3] infra/aws/Makefile not found at:
  ${makefile}

Expected operator flow:

  cd infra/aws
  make watch CAMPAIGN_ID=<id>

That target loops every ~5 minutes:
  collect (aws s3 sync) → merge → harness summarize → viz public/data/

Meanwhile, run the viz with live refresh:

  cd experiments/viz
  npm run sync-data && npm run dev
  # open http://localhost:5173/?live=1  (or use the Live toggle)

See infra/aws/README.md for lifecycle, S3 layout, and SSH_CIDR.
`);
  process.exit(1);
}

if (fs.existsSync(makefile)) {
  runMakeWatch();
} else {
  printFallback();
}
