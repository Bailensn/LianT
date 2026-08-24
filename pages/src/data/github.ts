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