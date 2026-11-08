import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useStore, deriveConversations, type Conversation } from '../store'
import { Page, Empty, formatTime, Badge } from '../ui/Common'

function stateText(s: string): { label: string; tone: 'ok' | 'warn' | 'muted' } {
  switch (s) {
    case 'online':
      return { label: '已连接', tone: 'ok' }
    case 'connecting':
      return { label: '连接中', tone: 'muted' }
    case 'reconnecting':
      return { label: '重连中', tone: 'warn' }
    case 'error':
      return { label: '连接失败', tone: 'warn' }
    default:
      return { label: '未连接', tone: 'muted' }
  }
}

function dayBucket(ts: number): string {
  const startOf = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime()
  const diff = Math.round((startOf(new Date()) - startOf(new Date(ts))) / 86_400_000)
  if (diff <= 0) return '今天'
  if (diff === 1) return '昨天'
  if (diff <= 7) return '本周'
  return '更早'
}

export default function Chats() {
  const msgs = useStore((s) => s.msgs)
  const read = useStore((s) => s.read)
  const remarks = useStore((s) => s.remarks)
  const state = useStore((s) => s.state)
  const detail = useStore((s) => s.detail)
  const bots = useStore((s) => s.bots)
  const reconnect = useStore((s) => s.reconnect)

  const [query, setQuery] = useState('')
  const allConvs = deriveConversations(msgs, read, remarks)
  const q = query.trim().toLowerCase()
  const convs = q
    ? allConvs.filter(
        (c) => c.title.toLowerCase().includes(q) || (c.last?.body?.plain ?? '').toLowerCase().includes(q),
      )
    : allConvs
  const st = stateText(state)
  const running = bots.filter((b) => b.status === 'running').length

  const groups = new Map<string, Conversation[]>()
  for (const c of convs) {
    const key = dayBucket(c.last?.ts ?? Date.now())
    const list = groups.get(key) ?? []
    list.push(c)
    groups.set(key, list)
  }

  return (
    <Page title="会话" action={<div className="pb-1"><Badge tone={st.tone}>{st.label}</Badge></div>}>
      <div className="mb-4 flex items-center gap-2 rounded-2xl bg-[var(--bg-elevated)] px-3">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--text-3)" strokeWidth="2" strokeLinecap="round">
          <circle cx="11" cy="11" r="7" />
          <path d="m16.5 16.5 4 4" />
        </svg>
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="搜索会话"
          className="h-11 min-w-0 flex-1 bg-transparent text-[15px] outline-none placeholder:text-[var(--text-3)]"
        />
      </div>

      {state !== 'online' && (
        <div className="mb-6 rounded-2xl bg-[var(--bg-elevated)] p-4">
          <p className="text-[15px] font-medium">与服务的连接中断</p>
          <p className="mt-1 text-[13px] leading-relaxed text-[var(--text-2)]">
            {detail || '正在尝试重新连接，收到的新消息会在恢复后补齐。'}
          </p>
          <button
            onClick={() => void reconnect()}
            className="mt-3 h-11 rounded-full bg-[var(--accent)] px-5 text-[14px] font-medium text-white active:opacity-70"
          >
            立即重试
          </button>
        </div>
      )}

      {bots.length === 0 && (
        <Link to="/bots" className="mb-6 block rounded-2xl bg-[var(--bg-elevated)] p-4 active:bg-[var(--bg-sunken)]">
          <p className="text-[15px] font-medium">还没有机器人</p>
          <p className="mt-1 text-[13px] text-[var(--text-2)]">添加一个 QQ 机器人后，收到消息就会出现在这里。</p>
        </Link>
      )}

      {convs.length === 0 ? (
        <Empty
          title="暂无会话"
          hint={running > 0 ? '机器人已在线，等待第一条消息' : '机器人尚未运行，先去「机器人」页面启动它'}
        />
      ) : (
        [...groups.entries()].map(([label, items]) => (
          <section key={label} className="mb-7">
            <h2 className="px-4 pb-2 text-[13px] font-medium tracking-wide text-[var(--text-2)]">{label}</h2>
            <div className="card overflow-hidden">
              {items.map((c, i) => (
                <div key={c.key}>
                  {i > 0 && <div className="ml-[68px] h-px bg-[var(--sep)]" />}
                  <Link to={`/chat/${encodeURIComponent(c.bot_id)}/${encodeURIComponent(c.peer_id)}`}>
                    <div className="flex min-h-[64px] items-center gap-3 px-4 py-3 transition active:bg-[var(--bg-sunken)] md:hover:bg-[var(--bg-sunken)]">
                      <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-[var(--bg-sunken)] text-[16px] font-semibold text-[var(--text-2)]">
                        {(c.title || '?').slice(0, 1).toUpperCase()}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-baseline justify-between gap-2">
                          <span className="truncate text-[16px] font-medium">{c.title}</span>
                          <span className="shrink-0 text-[12px] text-[var(--text-3)]">
                            {c.last ? formatTime(c.last.ts) : ''}
                          </span>
                        </div>
                        <div className="mt-0.5 flex items-center justify-between gap-2">
                          <span className="truncate text-[13px] text-[var(--text-2)]">
                            {c.last?.direction === 'out' ? '我：' : ''}
                            {c.last?.body?.plain || '空消息'}
                          </span>
                          {c.unread > 0 && (
                            <span className="flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-[var(--accent)] px-1 text-[11px] font-semibold text-white">
                              {c.unread > 99 ? '99+' : c.unread}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  </Link>
                </div>
              ))}
            </div>
          </section>
        ))
      )}
    </Page>
  )
}
