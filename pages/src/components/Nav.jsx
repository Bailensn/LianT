import { REPO_URL } from '../data/downloads.js'
import { SunIcon, MoonIcon, GithubMark } from './icons.jsx'

export default function Nav({ theme, onToggleTheme }) {
  return (
    <header className="nav">
      <div className="container">
        <div className="nav__inner glass">
          <a className="nav__brand" href="#top">
            <img src="/favicon.png" alt="" width="30" height="30" />
            <span className="nav__brand-name">
              LianT<span className="nav__brand-zh">· 联T</span>
            </span>
          </a>

          <nav className="nav__links" aria-label="主导航">
            <a className="nav__link" href="#architecture">
              架构
            </a>
            <a className="nav__link" href="#manager">
              Manager
            </a>
            <a className="nav__link" href="#service">
              Service
            </a>
            <a className="nav__link" href={REPO_URL} target="_blank" rel="noreferrer">
              GitHub
            </a>
          </nav>

          <div className="nav__right">
            <a
              className="nav__link"
              href={REPO_URL}
              target="_blank"
              rel="noreferrer"
              aria-label="在 GitHub 上查看源码"
              style={{ display: 'flex' }}
            >
              <GithubMark />
            </a>
            <button
              className="theme-toggle"
              onClick={onToggleTheme}
              aria-label={theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'}
            >
              {theme === 'dark' ? <SunIcon /> : <MoonIcon />}
            </button>
          </div>
        </div>
      </div>
    </header>
  )
}
