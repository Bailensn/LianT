export default function Architecture() {
  return (
    <section className="section" id="architecture">
      <div className="container">
        <div className="section__head reveal">
          <p className="eyebrow">架构</p>
          <h2 className="section__title">两个部分，各司其职</h2>
          <p className="section__desc">
            LianT 由两部分组成：Service 常驻在你自己的服务器上，直接对接 Telegram Bot API；Manager
            是你安装在电脑或手机上的图形界面，用来连接和操作 Service。数据始终留在你自己的服务器上。
          </p>
        </div>

        <div className="arch reveal reveal--delay-1">
          <div className="glass arch__card">
            <div className="arch__role">
              <h3 className="arch__name">Manager</h3>
              <span className="pill pill--cyan">客户端</span>
            </div>
            <p className="arch__desc">
              跨平台图形管理端，负责添加、配置和监控你的 Bot。支持桌面（Windows / Linux /
              macOS）与 Android。
            </p>
            <div className="arch__meta">
              <span className="pill">Apache-2.0</span>
            </div>
          </div>

          <div className="arch__link" aria-hidden="true">
            <span className="arch__link-line" />
            <span className="arch__link-label">网络连接</span>
            <span className="arch__link-line" />
          </div>

          <div className="glass arch__card">
            <div className="arch__role">
              <h3 className="arch__name">Service</h3>
              <span className="pill pill--violet">服务端</span>
            </div>
            <p className="arch__desc">
              部署在你自己服务器上的后台服务，常驻运行、对接 Telegram Bot API，负责实际收发和执行。
            </p>
            <div className="arch__meta">
              <span className="pill">AGPL-3.0</span>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
