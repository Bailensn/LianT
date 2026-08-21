import { ArrowIcon } from './icons.jsx'

export default function PlatformCard({ build, accent }) {
  const { os, arch, format, note, url } = build

  return (
    <div className={`glass glass--interactive card`}>
      <div className="card__top">
        <div>
          <div className="card__os">{os}</div>
          <div className="card__note">{note}</div>
        </div>
      </div>

      <div className="card__tags">
        <span className={`pill pill--${accent}`}>{arch}</span>
        <span className="pill">.{format}</span>
      </div>

      <a className="btn btn-ghost btn-sm card__cta" href={url} target="_blank" rel="noreferrer">
        下载 {os}（{format}）
        <ArrowIcon className="btn__arrow" />
      </a>
    </div>
  )
}
