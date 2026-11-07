import { useEffect } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { useStore } from './store'
import { BottomNav } from './ui/BottomNav'
import Chats from './pages/Chats'
import ChatPage from './pages/Chat'
import Bots from './pages/Bots'
import Settings from './pages/Settings'
import Pair from './pages/Pair'

export default function App() {
  const booted = useStore((s) => s.booted)
  const identity = useStore((s) => s.identity)
  const boot = useStore((s) => s.boot)

  useEffect(() => {
    void boot()
  }, [boot])

  if (!booted) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-[var(--text-3)] border-t-transparent" />
      </div>
    )
  }

  if (!identity) return <Pair />

  return (
    <div className="mx-auto flex h-full max-w-[520px] flex-col">
      <Routes>
        <Route path="/chats" element={<Chats />} />
        <Route path="/chat/:botId/:peerId" element={<ChatPage />} />
        <Route path="/bots" element={<Bots />} />
        <Route path="/settings" element={<Settings />} />
        <Route path="*" element={<Navigate to="/chats" replace />} />
      </Routes>
      <BottomNav />
    </div>
  )
}
