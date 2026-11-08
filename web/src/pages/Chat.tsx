import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { convKey, useStore } from '../store'
import { formatTime, Sheet, Button } from '../ui/Common'
import { RichBody } from '../ui/Body'

export default function Chat() {
  const { botId = '', peerId = '' } = useParams()
  const nav = useNavigate()
  const msgs = useStore((s) => s.msgs)
  const bots = useStore((s) => s.bots)
  const send = useStore((s) => s.send)
  const retrySend = useStore((s) => s.retrySend)
  const history = useStore((s) => s.history)
  const remarks = useStore((s) => s.remarks)
  const titles = useStore((s) => s.titles)
  const peerHints = useStore((s) => s.peerHints)
  const activeMode = useStore((s) => s.activeMode)
  const setActiveMode = useStore((s) => s.setActiveMode)
  const setRemark = useStore((s) => s.setRemark)
  const media = useStore((s) => s.media)
  const fetchMedia = useStore((s) => s.fetchMedia)
  const markRead = useStore((s) => s.markRead)
  const state = useStore((s) => s.state)

  const [text, setText] = useState('')
  const [collapsed, setCollapsed] = useState(false)
  const [historyError, setHistoryError] = useState('')
  const [loadingHistory, setLoadingHistory] = useState(false)
  const [editing, setEditing] = useState(false)
  const [remarkDraft, setRemarkDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const endRef = useRef<HTMLDivElement>(null)

  const bot = bots.find((b) => b.bot_id === botId)
  const list = useMemo(
    () => msgs.filter((m) => m.bot_id === botId && m.peer_id === peerId),
    [msgs, botId, peerId],
  )
  const key = convKey(botId, peerId)
  const known = remarks[key] || titles[key] || list.find((m) => m.sender_name)?.sender_name
  const title = known || shortPeer(peerId)
  const lastIn = [...list].reverse().find((m) => m.direction === 'in')

  useEffect(() => {
    setRemarkDraft(remarks[key] ?? '')
  }, [remarks, key])

  useEffect(() => {
    markRead(key)
  }, [key, list.length, markRead])

  const loadHistory = useCallback(async () => {
    setHistoryError('')
    setLoadingHistory(true)
    try {
      await history(botId, peerId)
    } catch (err) {
      setHistoryError((err as Error).message)
    } finally {
      setLoadingHistory(false)
    }
  }, [botId, peerId, history])

  useEffect(() => {
    void loadHistory()
  }, [loadHistory])

  useEffect(() => {
    endRef.current?.scrollIntoView({ block: 'end' })
  }, [list.length])

  // 非文本消息按需拉取签名地址，失败就留占位，不打断阅读。
  useEffect(() => {
    for (const m of list) {
      if (m.msg_type && m.msg_type !== 'text') void fetchMedia(m.bot_id, m.msg_id)
    }
  }, [list, fetchMedia])

  async function submit() {
    const value = text.trim()
    if (!value || !bot || sending) return
    // 输入框立刻清空，消息以「发送中」状态先上屏。
    setText('')
    setSending(true)
    setError('')
    try {
      await send(bot, peerId, lastIn?.peer_kind || 'private', value, lastIn?.msg_id)
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
      <Sheet open={editing} title="设置备注" onClose={() => setEditing(false)} dismissable>
        <div className="card mx-4 mb-4 overflow-hidden">
          <input
            value={remarkDraft}
            onChange={(e) => setRemarkDraft(e.target.value)}
            placeholder={known || shortPeer(peerId)}
            className="h-12 w-full bg-transparent px-4 text-[16px] outline-none placeholder:text-[var(--text-3)]"
          />
        </div>
        <label className="mx-4 mb-3 flex items-center justify-between rounded-2xl bg-[var(--bg-elevated)] px-4 py-3">
          <span className="text-[14px]">
            主动消息
            <span className="ml-2 text-[12px] text-[var(--text-3)]">不绑定原消息，也就没有 @ 和引用</span>
          </span>
          <input
            type="checkbox"
            checked={activeMode[key] === true}
            onChange={(e) => setActiveMode(key, e.target.checked)}
            className="h-5 w-5 accent-[var(--accent)]"
          />
        </label>
        <div className="px-4 pb-4">
          <Button
            full
            onClick={() => {
              setRemark(key, remarkDraft)
              setEditing(false)
            }}
          >
            保存备注
          </Button>
        </div>
      </Sheet>
      <header
        className={`glass sticky top-0 z-10 flex items-center gap-2 border-b px-2 transition-[padding] duration-200 ${
          collapsed ? 'border-[var(--sep)] py-1.5' : 'border-transparent pt-[max(0.5rem,env(safe-area-inset-top))] pb-2'
        }`}
      >
        <button
          onClick={() => nav(-1)}
          className="flex h-10 items-center gap-0.5 rounded-xl px-2 text-[var(--accent)] active:opacity-60 md:hidden"
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M15 5l-7 7 7 7" />
          </svg>
          <span className="text-[16px]">返回</span>
        </button>
        <div className="min-w-0 flex-1 text-center">
          <p className={`truncate font-semibold transition-all duration-200 ${collapsed ? 'text-[15px]' : 'text-[17px]'}`}>{title}</p>
          <p className={`truncate text-[11px] text-[var(--text-3)]`}>
            {peerHints[key] === 'whitelist' ? '群名需 QQ 白名单授权' : peerId}
          </p>
          <p className={`truncate overflow-hidden text-[11px] text-[var(--text-2)] transition-all duration-200 ${collapsed ? 'max-h-0 opacity-0' : 'max-h-4 opacity-100'}`}>
            {bot?.display_name || bot?.username || botId}
            {windowLeft !== null ? ` · ${
              windowLeft > 0
                ? `回复窗口 ${Math.floor(windowLeft / 60)}:${String(windowLeft % 60).padStart(2, '0')}`
                : '回复窗口已过'
            }` : ''}
          </p>
        </div>
        <button
          onClick={() => setEditing(true)}
          className="flex h-10 items-center rounded-xl px-2 text-[15px] text-[var(--accent)] active:opacity-60"
        >
          备注
        </button>
      </header>

      <div
        className="flex-1 overflow-y-auto px-4 py-4 no-scrollbar"
        onScroll={(e) => setCollapsed(e.currentTarget.scrollTop > 24)}
      >
        {loadingHistory && list.length === 0 && (
          <p className="mt-16 text-center text-[13px] text-[var(--text-3)]">正在加载历史…</p>
        )}
        {historyError && (
          <div className="mx-2 mt-4 rounded-2xl bg-[color-mix(in_oklab,var(--danger)_12%,transparent)] p-3 text-[13px] leading-relaxed text-[var(--danger)]">
            历史消息拉取失败：{historyError}
            <button onClick={() => void loadHistory()} className="ml-2 underline">
              重试
            </button>
          </div>
        )}
        {!loadingHistory && !historyError && list.length === 0 && (
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
                  className={`flex max-w-[78%] items-center gap-2 rounded-[18px] px-3.5 py-2 text-[15px] leading-relaxed whitespace-pre-wrap break-words ${
                    out ? 'bg-[var(--accent)] text-white' : 'bg-[var(--bg-elevated)]'
                  } ${m.failed ? 'opacity-60 ring-1 ring-[var(--danger)]' : ''}`}
                  onClick={() => m.failed && void retrySend(m.msg_id)}
                >
                  {m.failed && <span className="text-[12px]">发送失败 · 点击重试</span>}
                  {m.pending && (
                    <span className="h-3.5 w-3.5 shrink-0 animate-spin rounded-full border-2 border-white/40 border-t-white" />
                  )}
                  {m.msg_type && m.msg_type !== 'text' ? (
                    <MediaBody url={media[`${m.bot_id}/${m.msg_id}`]} fallback={m.body?.plain || '媒体消息'} />
                  ) : (
                    <RichBody doc={m.body?.doc as never} plain={m.body?.plain || '空消息'} />
                  )}
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
            onFocus={(e) => e.currentTarget.scrollIntoView({ block: 'center', behavior: 'smooth' })}
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

function MediaBody({ url, fallback }: { url?: string; fallback: string }) {
  if (url === undefined) {
    return <span className="opacity-70">正在加载媒体…</span>
  }
  if (!url) {
    return <span className="opacity-70">{fallback}</span>
  }
  return (
    <img
      src={url}
      alt={fallback}
      loading="lazy"
      className="max-h-64 w-auto max-w-full rounded-xl object-cover"
      onError={(e) => {
        e.currentTarget.replaceWith(Object.assign(document.createElement('span'), { textContent: '媒体已过期' }))
      }}
    />
  )
}

// 没有昵称时用 openid 后 6 位，别把整串 openid 糊在标题上。
function shortPeer(peer: string): string {
  const raw = peer.includes(':') ? peer.split(':').slice(1).join(':') : peer
  return raw.length > 8 ? `用户 ${raw.slice(-6)}` : `用户 ${raw}`
}
