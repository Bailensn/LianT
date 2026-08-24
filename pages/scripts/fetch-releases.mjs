import { writeFile, mkdir, readFile } from "node:fs/promises"
import { fileURLToPath } from "node:url"
import path from "node:path"

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const OUT = path.join(__dirname, "../src/data/releases.generated.json")

const repo = process.env.GITHUB_REPO || "LensnTeam/LianT"
const token = process.env.GITHUB_TOKEN || ""
const PRODUCTS = ["Manager", "Service"]

const headers = {
  Accept: "application/vnd.github+json",
  "User-Agent": "liant-pages-build",
  ...(token ? { Authorization: `Bearer ${token}` } : {}),
}

async function fetchReleases() {
  const res = await fetch(
    `https://api.github.com/repos/${repo}/releases?per_page=20`,
    { headers }
  )
  if (!res.ok) {
    throw new Error(
      `GitHub Releases 拉取失败：HTTP ${res.status} ${res.statusText}`
    )
  }
  const data = await res.json()
  return data.map((r) => ({
    id: r.id,
    tag_name: r.tag_name,
    name: r.name,
    published_at: r.published_at,
    assets: (r.assets || []).map((a) => ({
      name: a.name,
      browser_download_url: a.browser_download_url,
      size: a.size ?? undefined,
    })),
  }))
}

function pickLatest(releases, product) {
  return releases
    .filter((r) => r.tag_name.startsWith(`${product}-`))
    .sort(
      (a, b) =>
        new Date(b.published_at ?? 0).getTime() -
        new Date(a.published_at ?? 0).getTime()
    )[0]
}

async function main() {
  let releases
  try {
    releases = await fetchReleases()
  } catch (err) {
    console.warn(`⚠ ${err.message}`)
    try {
      const existing = JSON.parse(await readFile(OUT, "utf8"))
      console.log(`✓ 保留已有数据 ${OUT}`)
      return
    } catch {
      const payload = Object.fromEntries(PRODUCTS.map((p) => [p, null]))
      await mkdir(path.dirname(OUT), { recursive: true })
      await writeFile(OUT, JSON.stringify(payload, null, 2))
      console.log(`✓ 已写入空占位 ${OUT}`)
      return
    }
  }

  const payload = Object.fromEntries(
    PRODUCTS.map((p) => [p, pickLatest(releases, p) ?? null])
  )

  await mkdir(path.dirname(OUT), { recursive: true })
  await writeFile(OUT, JSON.stringify(payload, null, 2))

  const summary = PRODUCTS.map(
    (p) => `${p}: ${payload[p]?.tag_name ?? "(未找到)"}`
  ).join("  ")
  console.log(`✓ 已写入 ${OUT}`)
  console.log(` ${summary}`)
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})