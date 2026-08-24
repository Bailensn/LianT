import DownloadArch from "./DownloadArch";
import { getLatestRelease } from "@/data/github";
import type { Build } from "@/data/downloads";

interface Props {
  title: string
  description?: string
  builds: Build[]
  product: string
  num?: string
}

export default function DownloadSystem({ title, description, builds, product, num }: Props) {
  const release = getLatestRelease(product)

  return (
    <section className="section">
      <div className="container">
        <div className="page-meta">
          <div>
            <h1 className="section-title">
              <span className="num">{num ?? "00"}</span>
              {title}
            </h1>
            {description && <p className="muted">{description}</p>}
          </div>
          {release ? (
          <span className="pill">版本 {versionFromTag(release.name)}</span>
          ) : (
          <span className="pill pill-error">暂无Release数据</span>
          )}
        </div>

        <div className="systems">
          {builds.map((item) => (
            <DownloadArch
              key={item.os + item.arch}
              build={item}
              product={product}
              release={release}
            />
          ))}
        </div>
      </div>
    </section>
  )
}

export function versionFromTag(tag: string) {
  return tag.match(/v\d+\.\d+/)?.[0] ?? "None"
}