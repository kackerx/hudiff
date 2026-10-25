// usage: node npm/build.mjs <tag> <dir of the GoReleaser archives> <out dir>
// Makes the npm packages of a release: the four platform packages
// @kackerx/hudiff-<os>-<arch>, each with the binary of its archive, and the
// shell package hudiff that runs it, all at the tag's version.
import { execFileSync } from "node:child_process";
import { cpSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const [tag, archives, out] = process.argv.slice(2);
if (!out) {
  console.error("usage: node npm/build.mjs <tag> <archives dir> <out dir>");
  process.exit(2);
}
const version = tag.replace(/^v/, "");
const here = import.meta.dirname;
// [GOOS, GOARCH of the archive, process.platform, process.arch of Node]
const platforms = [
  ["darwin", "arm64", "darwin", "arm64"],
  ["darwin", "amd64", "darwin", "x64"],
  ["linux", "arm64", "linux", "arm64"],
  ["linux", "amd64", "linux", "x64"],
];

rmSync(out, { recursive: true, force: true });
const shell = JSON.parse(readFileSync(join(here, "hudiff", "package.json"), "utf8"));
shell.version = version;
shell.optionalDependencies = {};
for (const [goos, goarch, os, cpu] of platforms) {
  const name = `@kackerx/hudiff-${os}-${cpu}`;
  const archive = readdirSync(archives).find((f) => f.endsWith(`_${goos}_${goarch}.tar.gz`));
  if (!archive) throw new Error(`no archive for ${goos}_${goarch} in ${archives}`);
  const dir = join(out, `hudiff-${os}-${cpu}`);
  mkdirSync(join(dir, "bin"), { recursive: true });
  execFileSync("tar", ["-xzf", join(archives, archive), "-C", join(dir, "bin"), "hudiff"]); // keeps its mode
  const pkg = {
    name,
    version,
    description: `The ${os}-${cpu} binary of hudiff`,
    license: shell.license,
    repository: shell.repository,
    homepage: shell.homepage,
    os: [os],
    cpu: [cpu],
    files: ["bin", "LICENSE"],
  };
  cpSync(join(here, "..", "LICENSE"), join(dir, "LICENSE")); // MIT asks for the copy to travel with the binary
  writeFileSync(join(dir, "package.json"), JSON.stringify(pkg, null, 2) + "\n");
  shell.optionalDependencies[name] = version;
}
cpSync(join(here, "hudiff"), join(out, "hudiff"), { recursive: true });
cpSync(join(here, "..", "LICENSE"), join(out, "hudiff", "LICENSE"));
writeFileSync(join(out, "hudiff", "package.json"), JSON.stringify(shell, null, 2) + "\n");
