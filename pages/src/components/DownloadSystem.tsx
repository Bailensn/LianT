import DownloadArch from "./DownloadArch";
import { getLatestRelease } from "@/data/github";
import type { Build } from "@/data/downloads";

interface Props {
  title: string
  description?: string
  builds: Build[]
  /** 文件名与 tag 前缀中的产品名，例如 "Manager" / "Service" */
  product: string
  num?: string
}

/** 下载页容器：标题 + 版本徽标 + 系统卡片网格 */
export default function DownloadSystem({ title, description, builds, product, num }: Props) {
  // Release 数据在构建期由 scripts/fetch-releases.mjs 静态化，运行时直接读取
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
            <span className="pill">版本 {release.name}</span>
          ) : (
            <span className="pill pill-error">暂无 Release 数据</span>
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
