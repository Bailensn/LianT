"use client";

import SystemIcon from "./SystemIcon";
import { buildReleaseFileName, type Build } from "@/data/downloads";
import type { Release } from "@/data/github";

interface Props {
  build: Build
  product: string
  release: Release | undefined
}

function getUrl(build: Build, product: string, release: Release | undefined, ext: string) {
  if (!release) return undefined
  const filename = buildReleaseFileName({
    product,
    tag: release.tag_name,
    os: build.os,
    arch: build.arch,
    ext,
  })
  const found = release.assets.find((a) => a.name === filename)
  return found?.browser_download_url
}

/** 单个系统/架构的下载卡片 */
export default function DownloadArch({ build, product, release }: Props) {
  const assetLinks = build.assets.map((asset) => ({
    asset,
    url: getUrl(build, product, release, asset.ext),
  }))
  const hasDownload = assetLinks.some((item) => item.url)

  return (
    <div className={hasDownload ? "card" : "card card-empty"}>
      <div className="card-head">
        <div className="card-icon">
          <SystemIcon os={build.os} size={32} />
        </div>
        <div>
          <h2>{build.os}</h2>
          <h3>{build.arch}</h3>
        </div>
      </div>

      <div className="formats">
        {assetLinks.map((item, i) => (
          <a
            key={item.asset.ext + i}
            href={item.url ?? "#"}
            className={item.url ? "" : "disabled"}
            aria-disabled={!item.url}
            onClick={(e) => {
              if (!item.url) e.preventDefault()
            }}
          >
            {item.asset.label}
          </a>
        ))}
      </div>
    </div>
  )
}
