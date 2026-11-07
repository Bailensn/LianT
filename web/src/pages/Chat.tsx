import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { convKey, useStore } from '../store'
import { formatTime } from '../ui/Common'

export default function Chat() {
  const { botId = '', peerId = '' } = useParams()
  const nav = useNavigate()
  const msgs = useStore((s) => s.msgs)
  const bots = useStore((s) => s.bots)
  const send = useStore((s) => s.send)
  const history = useStore((s) => s.history)
  const markRead = useStore((s) => s.markRead)
  const state = useStore((s) => s.state)

  const [text, setText] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const endRef = useRef<HTMLDivElement>(null)

  const bot = bots.find((b) => b.bot_id === botId)
  const list = useMemo(
    () => msgs.filter((m) => m.bot_id === botId && m.peer_id === peerId),
    [msgs, botId, peerId],
  )
  const key = convKey(botId, peerId)
  const title = list.find((m) => m.sender_name)?.sender_name || peerId
  const lastIn = [...list].reverse().find((m) => m.direction === 'in')

  useEffect(() => {
    markRead(key)
  }, [key, list.length, markRead])

  useEffect(() => {
    void history(botId, peerId)
  }, [botId, peerId, history])

  useEffect(() => {
    endRef.current?.scrollIntoView({ block: 'end' })
  }, [list.length])

  async function submit() {
    const value = text.trim()
    if (!value || !bot || sending) return
    setSending(true)
    setError('')
    try {
      await send(bot, peerId, lastIn?.peer_kind || 'private', value, lastIn?.msg_id)
      setText('')
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setSending(false)
    }
  }

  // QQ 被动回复有 5 分钟窗口；过期后仍然允许发送，只是提醒可能失败。
  const windowLeft = bot?.constraints?.reply_window_s && lastIn
    ? Math.max(0, Math.round((lastIn.ts + bot.constraints.reply_window_s * 1000 - Date.now()) / 1000))
    : null

  return (
    <div className="flex h-full flex-col">
      <header className="glass sticky top-0 z-10 flex items-center gap-2 border-b border-[var(--sep)] px-2 pt-[max(0.5rem,env(safe-area-inset-top))] pb-2">
        <button
          onClick={() => nav(-1)}
          className="flex h-10 items-center gap-0.5 rounded-xl px-2 text-[var(--accent)] active:opacity-60"
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M15 5l-7 7 7 7" />
          </svg>
          <span className="text-[16px]">返回</span>
        </button>
        <div className="min-w-0 flex-1 text-center">
          <p className="truncate text-[16px] font-semibold">{title}</p>
          <p className="truncate text-[11px] text-[var(--text-2)]">
            {bot?.display_name || bot?.username || botId}
            {windowLeft !== null ? ` · ${
              windowLeft > 0
                ? `回复窗口 ${Math.floor(windowLeft / 60)}:${String(windowLeft % 60).padStart(2, '0')}`
                : '回复窗口已过'
            }` : ''}
          </p>
        </div>
        <div className="w-[68px]" />
      </header>

      <div className="flex-1 overflow-y-auto px-4 py-4 no-scrollbar">
        {list.length === 0 && (
          <p className="mt-16 text-center text-[13px] text-[var(--text-3)]">还没有消息</p>
        )}
        {list.map((m, i) => {
          const prev = list[i - 1]
          const gap = prev ? m.ts - prev.ts > 5 * 60_000 : true
          const out = m.direction === 'out'
          return (
            <div key={m.msg_id}>
              {gap && (
                <p className="my-3 text-center text-[11px] text-[var(--text-3)]">{formatTime(m.ts)}</p>
              )}
              <div className={`flex ${out ? 'justify-end' : 'justify-start'} mb-1.5`}>
                <div
                  className={`max-w-[78%] rounded-[18px] px-3.5 py-2 text-[15px] leading-relaxed whitespace-pre-wrap break-words ${
                    out ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg-elevated)]'
                  }`}
                >
                  {m.body?.plain || '空消息'}
                </div>
              </div>
            </div>
          )
        })}
        <div ref={endRef} className="h-2" />
      </div>

      {error && <p className="px-4 pb-1 text-[12px] text-[var(--danger)]">{error}</p>}

      <div className="safe-bottom px-3 pb-3">
        <div className="flex items-end gap-2 rounded-[22px] border border-[var(--sep)] bg-[var(--bg-elevated)] p-1.5">
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
                e.preventDefault()
                void submit()
              }
            }}
            rows={1}
            placeholder={state === 'online' ? '发消息…' : '连接中…'}
            className="max-h-32 min-h-[38px] flex-1 resize-none bg-transparent px-2.5 py-2 text-[16px] leading-snug outline-none placeholder:text-[var(--text-3)]"
          />
          <button
            onClick={submit}
            disabled={!text.trim() || sending || state !== 'online'}
            aria-label="发送"
            className="mb-0.5 flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-[var(--accent)] text-white transition active:scale-95 disabled:opacity-30"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round">
              <path d="M12 19V5M5 12l7-7 7 7" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  )
}
