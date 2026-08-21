import { useEffect, useState } from 'react'
import Ambient from './components/Ambient.jsx'
import Nav from './components/Nav.jsx'
import Hero from './components/Hero.jsx'
import Architecture from './components/Architecture.jsx'
import DownloadSection from './components/DownloadSection.jsx'
import Footer from './components/Footer.jsx'
import { serviceBuilds, managerBuilds } from './data/downloads.js'

function getInitialTheme() {
  if (typeof window === 'undefined') return 'dark'
  const saved = window.localStorage.getItem('liant-theme')
  if (saved === 'dark' || saved === 'light') return saved
  const prefersLight = window.matchMedia?.('(prefers-color-scheme: light)').matches
  return prefersLight ? 'light' : 'dark'
}

export default function App() {
  const [theme, setTheme] = useState(getInitialTheme)

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    window.localStorage.setItem('liant-theme', theme)
  }, [theme])

  const toggleTheme = () => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))

  return (
    <>
      <Ambient />
      <div className="page">
        <Nav theme={theme} onToggleTheme={toggleTheme} />
        <Hero />
        <Architecture />

        <DownloadSection
          id="manager"
          eyebrow="Manager · 客户端"
          title="下载 Manager"
          desc="图形管理端，用来连接你的 Service 并管理 Bot。选择你的设备平台。"
          builds={managerBuilds}
          accent="cyan"
        />

        <DownloadSection
          id="service"
          eyebrow="Service · 服务端"
          title="下载 Service"
          desc="部署在你自己服务器上的后台服务。根据服务器的系统和架构选择对应安装包。"
          builds={serviceBuilds}
          accent="violet"
        />

        <Footer />
      </div>
    </>
  )
}
