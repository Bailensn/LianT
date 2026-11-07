import { useStore, deriveConversations } from '../store'
import { Page, Empty, formatTime, Badge } from '../ui/Common'
import { Link } from 'react-router-dom'

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

export default function Chats() {
  const msgs = useStore((s) => s.msgs)
  const read = useStore((s) => s.read)
  const state = useStore((s) => s.state)
  const detail = useStore((s) => s.detail)
  const bots = useStore((s) => s.bots)
  const reconnect = useStore((s) => s.reconnect)
  const convs = deriveConversations(msgs, read)
  const st = stateText(state)
  const running = bots.filter((b) => b.status === 'running').length

  return (
    <Page
      title="会话"
      action={
        <div className="pb-1">
          <Badge tone={st.tone}>{st.label}</Badge>
        </div>
      }
    >
      {state !== 'online' && (
        <div className="mb-4 rounded-2xl bg-[var(--bg-elevated)] p-4">
          <p className="text-[15px] font-medium">与服务的连接中断</p>
          <p className="mt-1 text-[13px] leading-relaxed text-[var(--text-2)]">
            {detail || '正在尝试重新连接，收到的新消息会在恢复后补齐。'}
          </p>
          <button
            onClick={() => void reconnect()}
            className="mt-3 h-9 rounded-full bg-[var(--accent)] px-4 text-[13px] font-medium text-white active:opacity-70"
          >
            立即重试
          </button>
        </div>
      )}

      {bots.length === 0 && (
        <Link to="/bots" className="mb-4 block rounded-2xl bg-[var(--bg-elevated)] p-4 active:bg-[var(--bg-sunken)]">
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
        <div className="card mb-4 overflow-hidden">
          {convs.map((c, i) => (
            <div key={c.key}>
              {i > 0 && <div className="ml-[68px] h-px bg-[var(--sep)]" />}
              <Link to={`/chat/${encodeURIComponent(c.bot_id)}/${encodeURIComponent(c.peer_id)}`}>
                <div className="flex items-center gap-3 px-4 py-3 active:bg-[var(--bg-sunken)]">
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
      )}
    </Page>
  )
}
