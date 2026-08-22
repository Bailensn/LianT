export interface ReleaseAsset {
  name: string
  browser_download_url: string
  size?: number
}

export interface Release {
  tag_name: string
  name: string
  published_at: string | null
  assets: ReleaseAsset[]
}

export async function getLatestRelease(product: string) {
  const releases = await getReleases()
  // tag 形如 "Manager-v0.01" / "Service-v0.01"，按产品前缀筛选
  const list = releases
    .filter(r => r.tag_name.startsWith(`${product}-`))
    .sort((a, b) =>
      new Date(b.published_at ?? 0).getTime() -
      new Date(a.published_at ?? 0).getTime()
    )
  return list[0]
}

const cache = new Map<string, Promise<Release[]>>()

function getReleases() {
  const repo =
    (import.meta as any).env?.VITE_GITHUB_REPO ??
    "Bailensn/LianT"

  const url = `https://api.github.com/repos/${repo}/releases?per_page=20`

  if (!cache.has(url)) {
    cache.set(url, fetchReleases(url))
  }
  return cache.get(url)!
}

async function fetchReleases(url: string) {
  const res = await fetch(url)

  if (!res.ok) {
    throw new Error("Release 获取失败")
  }

  return await res.json() as Release[]
}