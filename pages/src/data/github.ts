export interface ReleaseAsset {
  name: string
  browser_download_url: string
}

export interface Release {
  tag_name: string
  name: string
  assets: ReleaseAsset[]
}

export async function getRelease(tag: string) {
  const repo =
    (import.meta as any).env?.VITE_GITHUB_REPO ??
    "Bailensn/LianT"

  const url =
    `https://api.github.com/repos/${repo}/releases/tags/${tag}`

  if (!cache.has(url)) {
    cache.set(url, fetchRelease(url))
  }
  return await cache.get(url)!
}

const cache = new Map<string, Promise<Release>>()

async function fetchRelease(url: string) {
  const res = await fetch(url)

  if (!res.ok) {
    throw new Error("Release 获取失败")
  }

  return await res.json() as Release
}