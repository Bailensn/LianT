import { useEffect } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { useStore } from './store'
import { BottomNav } from './ui/BottomNav'
import { DesktopSidebar } from './ui/DesktopSidebar'
import Chats from './pages/Chats'
import ChatPage from './pages/Chat'
import Bots from './pages/Bots'
import Settings from './pages/Settings'
import Pair from './pages/Pair'

export default function App() {
  const booted = useStore((s) => s.booted)
  const identity = useStore((s) => s.identity)
  const boot = useStore((s) => s.boot)
  const { pathname } = useLocation()

  useEffect(() => {
    void boot()
  }, [boot])

  // 第一次交互时申请通知权限，后台也能收到提醒。
  useEffect(() => {
    if (typeof Notification === 'undefined' || Notification.permission !== 'default') return
    const ask = () => {
      void Notification.requestPermission()
      window.removeEventListener('pointerdown', ask)
    }
    window.addEventListener('pointerdown', ask, { once: true })
    return () => window.removeEventListener('pointerdown', ask)
  }, [])

  const state = useStore((s) => s.state)
  const reconnect = useStore((s) => s.reconnect)
  useEffect(() => {
    const onVisible = () => {
      if (!document.hidden && state !== 'online') void reconnect()
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => document.removeEventListener('visibilitychange', onVisible)
  }, [state, reconnect])

  if (!booted) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-[var(--text-3)] border-t-transparent" />
      </div>
    )
  }

  if (!identity) return <Pair />

  const inChat = pathname.startsWith('/chat/')
  const isChatArea = pathname === '/chats' || inChat

  return (
    <div className="flex h-full">
      <DesktopSidebar />

      <div className="flex min-w-0 flex-1">
        {isChatArea ? (
          <>
            <div
              className={`${
                inChat ? 'hidden md:flex' : 'flex'
              } h-full w-full min-w-0 flex-col md:w-[360px] md:max-w-[360px] md:border-r md:border-[var(--sep)]`}
            >
              <Chats />
            </div>
            <div className={`${inChat ? 'flex' : 'hidden md:flex'} h-full min-w-0 flex-1 flex-col`}>
              {inChat ? (
                <Routes>
                  <Route path="/chat/:botId/:peerId" element={<ChatPage />} />
                </Routes>
              ) : (
                <div className="flex h-full flex-col items-center justify-center text-center">
                  <div className="mb-3 h-14 w-14 rounded-[18px] bg-[var(--bg-sunken)]" />
                  <p className="text-[15px] font-medium">选择一个会话</p>
                  <p className="mt-1 text-[13px] text-[var(--text-2)]">消息会实时出现在这里</p>
                </div>
              )}
            </div>
          </>
        ) : (
          <div className="h-full w-full min-w-0">
            <Routes>
              <Route path="/bots" element={<Bots />} />
              <Route path="/settings" element={<Settings />} />
              <Route path="*" element={<Navigate to="/chats" replace />} />
            </Routes>
          </div>
        )}
      </div>

      <BottomNav />
    </div>
  )
}
