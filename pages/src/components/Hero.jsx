import { REPO_URL } from '../data/downloads.js'
import { ArrowIcon } from './icons.jsx'

export default function Hero() {
  return (
    <section className="hero-section" id="top">
      <div className="container">
        <div className="hero">
          <div className="hero__copy reveal">
            <p className="eyebrow">Self-hosted · Telegram Bot 管理</p>
            <h1 className="hero__title">
              LianT<span className="hero__dot">·</span>
              <span className="hero__title-zh">联T</span>
            </h1>
            <p className="hero__tagline">
              一款基于 Telegram Bot API 的管理工具，服务器自备。Service
              运行在你自己的服务器上，Manager 是跨平台的图形管理端——你的 Bot，你的服务器，你的掌控。
            </p>
            <div className="hero__ctas">
              <a className="btn btn-primary" href="#manager">
                获取 Manager
                <ArrowIcon className="btn__arrow" />
              </a>
              <a className="btn btn-ghost" href="#service">
                获取 Service
                <ArrowIcon className="btn__arrow" />
              </a>
              <a className="hero__source-link" href={REPO_URL} target="_blank" rel="noreferrer">
                查看源码 →
              </a>
            </div>
          </div>

          <div className="hero__art reveal reveal--delay-2">
            <div className="hero__portal">
              <img className="float" src="/logo.png" alt="LianT 图标" width="512" height="512" />
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
