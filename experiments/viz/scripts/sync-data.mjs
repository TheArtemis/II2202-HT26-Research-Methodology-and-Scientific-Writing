import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, "../..");
const repoRoot = path.resolve(root, "..");
const destDir = path.resolve(__dirname, "../public/data");
const dest = path.join(destDir, "dataset.jsonl");
const statusDest = path.join(destDir, "campaign-status.json");

const candidates = [
  process.argv[2],
  process.env.DATASET_JSONL,
  path.join(repoRoot, "results/campaigns/full-20261007A/merged/dataset.jsonl"),
  path.join(repoRoot, "results/campaigns/full-20261001A/merged/dataset.jsonl"),
  path.join(repoRoot, "results/campaigns/full-20261001/merged/dataset.jsonl"),
  path.join(root, "results/pilot/dataset.jsonl"),
  path.join(root, "results/smoke/dataset.jsonl"),
  path.join(root, "results/full/dataset.jsonl"),
].filter(Boolean);

fs.mkdirSync(destDir, { recursive: true });

let src = null;
for (const c of candidates) {
  if (c && fs.existsSync(c)) {
    src = c;
    break;
  }
}
if (!src) {
  console.error("No dataset.jsonl found. Run: harness summarize results/<batch>");
  process.exit(1);
}
fs.copyFileSync(src, dest);
console.log(`synced ${src} → ${dest}`);

const statusCandidates = [
  path.join(path.dirname(src), "campaign-status.json"),
  path.join(repoRoot, "results/campaigns/full-20261007A/merged/campaign-status.json"),
  path.join(repoRoot, "results/campaigns/full-20261001A/merged/campaign-status.json"),
  path.join(repoRoot, "results/campaigns/full-20261001/merged/campaign-status.json"),
];
for (const c of statusCandidates) {
  if (c && fs.existsSync(c)) {
    fs.copyFileSync(c, statusDest);
    console.log(`synced ${c} → ${statusDest}`);
    break;
  }
}