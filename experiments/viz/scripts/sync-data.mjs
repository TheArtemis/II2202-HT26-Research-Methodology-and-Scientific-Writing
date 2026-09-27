import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, "../..");
const destDir = path.resolve(__dirname, "../public/data");
const dest = path.join(destDir, "dataset.jsonl");

const candidates = [
  process.argv[2],
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
