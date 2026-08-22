// 该文件在构建期由 scripts/fetch-releases.mjs 生成，
// 运行时前端仅读取静态数据，不再调用 GitHub API。
import generated from "./releases.generated.json"

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

type GeneratedReleases = Record<string, Release | null>

export function getLatestRelease(product: string): Release | undefined {
  return (generated as GeneratedReleases)[product] ?? undefined
}