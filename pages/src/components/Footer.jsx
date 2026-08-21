import { REPO_URL } from '../data/downloads.js'

export default function Footer() {
  return (
    <footer className="footer">
      <div className="container">
        <div className="footer__top">
          <div className="nav__brand">
            <img src="/favicon.png" alt="" width="26" height="26" />
            <span className="nav__brand-name">
              LianT<span className="nav__brand-zh">· 联T</span>
            </span>
          </div>

          <div className="footer__licenses">
            <span className="pill">根目录 / Service：AGPL-3.0</span>
            <span className="pill">Manager：Apache-2.0</span>
          </div>

          <a className="hero__source-link" href={REPO_URL} target="_blank" rel="noreferrer">
            {REPO_URL.replace('https://', '')}
          </a>
        </div>

        <div className="footer__bottom">
          <p className="footer__credit">
            LianT by Bailensn
            <br />
            Powered by Claude
          </p>
        </div>
      </div>
    </footer>
  )
}
