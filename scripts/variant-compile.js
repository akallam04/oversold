"use strict"

const fs = require("fs")
const path = require("path")
const { execFileSync } = require("child_process")

const [src, out, ...pkgDirs] = process.argv.slice(2)
if (!src || !out || pkgDirs.length === 0) {
  console.error("usage: node variant-compile.js <medusa source> <out dir> <package dir>...")
  process.exit(2)
}

const serverModules = "/server/node_modules"
const tools = "/tmp/variant-tools"
const build = "/tmp/variant-build"

function lockVersion(lock, name, range) {
  const header = `${name}@npm:${range}`
  const lines = lock.split("\n")
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (line.startsWith(" ") || !line.endsWith(":")) {
      continue
    }
    const keys = line.slice(0, -1).replace(/"/g, "").split(", ")
    if (!keys.includes(header)) {
      continue
    }
    for (let j = i + 1; j < lines.length && lines[j].startsWith(" "); j++) {
      const m = lines[j].match(/^  version: (\S+)$/)
      if (m) {
        return m[1]
      }
    }
  }
  throw new Error(`yarn.lock has no entry for ${header}`)
}

function run(cmd, args, cwd) {
  execFileSync(cmd, args, { cwd, stdio: "inherit" })
}

const rootPkg = JSON.parse(fs.readFileSync(path.join(src, "package.json"), "utf8"))
const lock = fs.readFileSync(path.join(src, "yarn.lock"), "utf8")
const dev = rootPkg.devDependencies
const versions = {
  typescript: lockVersion(lock, "typescript", dev.typescript),
  "tsc-alias": lockVersion(lock, "tsc-alias", dev["tsc-alias"]),
  "@types/node": lockVersion(lock, "@types/node", dev["@types/node"]),
  "@types/jest": lockVersion(lock, "@types/jest", "*"),
  "expect-type": lockVersion(lock, "expect-type", dev["expect-type"]),
}
console.log(`versions from Medusa's yarn.lock: ${JSON.stringify(versions)}`)

const imageTs = require(path.join(serverModules, "typescript/package.json")).version
const tsDir = imageTs === versions.typescript ? path.join(serverModules, "typescript") : path.join(tools, "node_modules/typescript")
const wanted = ["tsc-alias", "@types/node", "@types/jest", "expect-type"]
if (tsDir !== path.join(serverModules, "typescript")) {
  wanted.push("typescript")
}
fs.mkdirSync(tools, { recursive: true })
fs.writeFileSync(path.join(tools, "package.json"), JSON.stringify({ name: "variant-tools", private: true }))
run("npm", ["install", "--no-audit", "--no-fund", "--save-exact", ...wanted.map((n) => `${n}@${versions[n]}`)], tools)
const tsc = path.join(tsDir, "bin/tsc")
const tscAlias = path.join(tools, "node_modules/tsc-alias/dist/bin/index.js")
console.log(`typescript ${require(path.join(tsDir, "package.json")).version} from ${tsDir}`)

fs.rmSync(build, { recursive: true, force: true })
fs.mkdirSync(build, { recursive: true })
fs.copyFileSync(path.join(src, "_tsconfig.base.json"), path.join(build, "_tsconfig.base.json"))
const modules = path.join(build, "node_modules")
fs.mkdirSync(path.join(modules, "@types"), { recursive: true })
for (const entry of fs.readdirSync(serverModules)) {
  if (entry === "@types" || entry === "expect-type" || entry.startsWith(".")) {
    continue
  }
  fs.symlinkSync(path.join(serverModules, entry), path.join(modules, entry))
}
for (const entry of fs.readdirSync(path.join(serverModules, "@types"))) {
  if (entry === "node" || entry === "jest") {
    continue
  }
  fs.symlinkSync(path.join(serverModules, "@types", entry), path.join(modules, "@types", entry))
}
for (const entry of ["node", "jest"]) {
  fs.symlinkSync(path.join(tools, "node_modules/@types", entry), path.join(modules, "@types", entry))
}
fs.symlinkSync(path.join(tools, "node_modules/expect-type"), path.join(modules, "expect-type"))

for (const dir of pkgDirs) {
  const from = path.join(src, dir)
  const to = path.join(build, dir)
  fs.cpSync(from, to, { recursive: true, filter: (p) => !/\/(dist|node_modules)(\/|$)/.test(p.slice(from.length)) })
  const pkg = JSON.parse(fs.readFileSync(path.join(to, "package.json"), "utf8"))
  console.log(`build ${pkg.name}: ${pkg.scripts.build}`)
  run("node", [tsc, "--build"], to)
  if (pkg.scripts.build.includes("resolve:aliases")) {
    const resolved = execFileSync("node", [tsc, "--showConfig", "-p", "tsconfig.json"], { cwd: to })
    fs.writeFileSync(path.join(to, "tsconfig.resolved.json"), resolved)
    run("node", [tscAlias, "-p", "tsconfig.resolved.json"], to)
    fs.rmSync(path.join(to, "tsconfig.resolved.json"))
  }
  const dest = path.join(out, pkg.name)
  fs.rmSync(dest, { recursive: true, force: true })
  for (const rel of fs.readdirSync(path.join(to, "dist"), { recursive: true })) {
    const file = path.join(to, "dist", rel)
    if (fs.statSync(file).isFile()) {
      fs.mkdirSync(path.dirname(path.join(dest, "dist", rel)), { recursive: true })
      fs.writeFileSync(path.join(dest, "dist", rel), fs.readFileSync(file))
    }
  }
}
