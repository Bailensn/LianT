import Link from "next/link";

const steps = [
  { num: "01", title: "选择组件", desc: "Manager 或 Service" },
  { num: "02", title: "选择平台", desc: "系统与架构" },
  { num: "03", title: "直接下载", desc: "GitHub 直链" },
];

export default function HomePage() {
  return (
    <main>
      <section className="hero">
        <div className="container hero-grid">
          <div className="hero-copy">
            <p className="eyebrow">‖ 你好，这里是</p>
            <h1>
              LianT <span className="hero-accent">联T</span>
            </h1>
            <p className="lead">Telegram Bot 管理器与服务端组件。服务器自备。</p>

            <div className="hero-cta">
              <Link className="btn btn-primary" href="/manager">
                <span className="btn-icon">↓</span>
                获取 Manager
              </Link>
              <Link className="btn btn-ghost" href="/service">
                获取 Service
                <span className="btn-arrow">→</span>
              </Link>
            </div>
          </div>

          <div className="code-window" aria-hidden="true">
            <div className="code-header">
              <span className="dot red" />
              <span className="dot yellow" />
              <span className="dot green" />
              <span className="code-title">quick-start.txt</span>
            </div>
            <pre className="code-body">
              <code>
                <span className="code-comment"># 快速开始</span>
                <span className="code-line">
                  <span className="code-num">1.</span> 选择要下载的组件
                </span>
                <span className="code-line">
                  <span className="code-num">2.</span> 选系统与架构
                </span>
                <span className="code-line">
                  <span className="code-num">3.</span> 一键直链下载
                </span>
                {"\n"}
                <span className="code-tag">.exe</span>{" "}
                <span className="code-tag">.deb</span>{" "}
                <span className="code-tag">.rpm</span>{" "}
                <span className="code-tag">.apk</span>
              </code>
            </pre>
          </div>
        </div>
      </section>

      <section className="section">
        <div className="container">
          <h2 className="section-title">
            <span className="num">01</span>
            如何使用
          </h2>

          <div className="steps">
            {steps.map((step) => (
              <div key={step.num} className="step-card">
                <span className="step-num">{step.num}</span>
                <div>
                  <h3>{step.title}</h3>
                  <p>{step.desc}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>
    </main>
  );
}
