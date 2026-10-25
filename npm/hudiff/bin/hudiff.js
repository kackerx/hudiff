#!/usr/bin/env node
// Runs the hudiff binary of the platform package npm installed beside this
// one (optionalDependencies, as esbuild does it): no postinstall download, so
// it works where scripts are off (pnpm 10, --ignore-scripts).
const { spawnSync } = require("node:child_process");

const name = `@kackerx/hudiff-${process.platform}-${process.arch}`;
let bin;
try {
  bin = require.resolve(`${name}/bin/hudiff`);
} catch {
  console.error(
    `hudiff: no binary for ${process.platform}-${process.arch}: ${name} is not installed.\n` +
      "It is an optional dependency: install again without --no-optional / --omit=optional,\n" +
      "or build from source: go install github.com/kackerx/hudiff@latest",
  );
  process.exit(1);
}
const r = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
if (r.error) {
  console.error(`hudiff: ${r.error.message}`);
  process.exit(1);
}
if (r.signal) process.kill(process.pid, r.signal); // killed as the child was
process.exit(r.status ?? 1);
