import { useState } from 'react'
import { useStore, type Bot } from '../store'
import { Page, Group, Divider, Field, Badge, Empty, Spinner } from '../ui/Common'

export default function Bots() {
  const bots = useStore((s) => s.bots)
  const addBot = useStore((s) => s.addBot)
  const startBot = useStore((s) => s.startBot)
  const stopBot = useStore((s) => s.stopBot)
  const removeBot = useStore((s) => s.removeBot)
  const state = useStore((s) => s.state)

  const [adding, setAdding] = useState(false)
  const [appId, setAppId] = useState('')
  const [secret, setSecret] = useState('')
  const [label, setLabel] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [pendingId, setPendingId] = useState('')

  async function act(id: string, fn: (id: string) => Promise<void>) {
    setPendingId(id)
    setError('')
    try {
      await fn(id)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setPendingId('')
    }
  }

  async function submit() {
    if (!appId.trim() || !secret.trim()) return
    setBusy(true)
    setError('')
    try {
      await addBot('qqbot', { app_id: appId.trim(), app_secret: secret.trim() }, label.trim())
      setAdding(false)
      setAppId('')
      setSecret('')
      setLabel('')
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Page
      title="机器人"
      action={
        <button
          onClick={() => setAdding(true)}
          className="mb-1 flex h-9 items-center gap-1 rounded-full bg-[var(--bg-elevated)] px-3.5 text-[14px] font-medium text-[var(--accent)] active:opacity-70"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round">
            <path d="M12 5v14M5 12h14" />
          </svg>
          添加
        </button>
      }
    >
      {error && (
        <div className="mb-4 rounded-2xl bg-[color-mix(in_oklab,var(--danger)_12%,transparent)] p-3 text-[13px] leading-relaxed text-[var(--danger)]">
          {error}
        </div>
      )}

      {bots.length === 0 ? (
        <Empty title="还没有机器人" hint="点右上角「添加」，填入 QQ 开放平台的 AppID 与 AppSecret" />
      ) : (
        <Group title="已配置" footer="状态由服务端推送；停止后不再接收新消息，历史仍保留。">
          {bots.map((b, i) => (
            <div key={b.bot_id}>
              {i > 0 && <Divider />}
              <BotRow
                bot={b}
                busy={pendingId === b.bot_id}
                online={state === 'online'}
                onToggle={() => act(b.bot_id, b.status === 'running' ? stopBot : startBot)}
                onRemove={() => {
                  if (confirm(`移除 ${b.display_name || b.username || b.bot_id}？`)) {
                    void act(b.bot_id, removeBot)
                  }
                }}
              />
            </div>
          ))}
        </Group>
      )}

      {adding && (
        <div className="fixed inset-0 z-40 flex items-end bg-black/40" onClick={() => setAdding(false)}>
          <div
            className="w-full rounded-t-[28px] bg-[var(--bg)] pb-[max(1rem,env(safe-area-inset-bottom))]"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mx-auto mt-2 h-1 w-9 rounded-full bg-[var(--text-3)]/40" />
            <div className="flex items-center justify-between px-5 py-3">
              <button onClick={() => setAdding(false)} className="text-[16px] text-[var(--text-2)]">
                取消
              </button>
              <p className="text-[16px] font-semibold">添加 QQ 机器人</p>
              <button
                onClick={submit}
                disabled={busy || !appId.trim() || !secret.trim()}
                className="text-[16px] font-semibold text-[var(--accent)] disabled:opacity-40"
              >
                {busy ? '验证中' : '保存'}
              </button>
            </div>
            <div className="card mx-4 mb-4 overflow-hidden">
              <Field label="AppID" value={appId} onChange={setAppId} placeholder="1020xxxxx" mono />
              <Divider />
              <Field label="AppSecret" value={secret} onChange={setSecret} placeholder="平台密钥" type="password" mono />
              <Divider />
              <Field label="备注名 · 可留空" value={label} onChange={setLabel} placeholder="例如 客服号" />
            </div>
            <p className="px-5 pb-2 text-[12px] leading-relaxed text-[var(--text-3)]">
              保存时会向 QQ 开放平台校验凭据并读取机器人昵称；密钥只存在服务端。
            </p>
          </div>
        </div>
      )}
    </Page>
  )
}

function BotRow({
  bot,
  busy,
  online,
  onToggle,
  onRemove,
}: {
  bot: Bot
  busy: boolean
  online: boolean
  onToggle: () => void
  onRemove: () => void
}) {
  const running = bot.status === 'running'
  return (
    <div className="px-4 py-3">
      <div className="flex items-center gap-3">
        <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-[14px] bg-[var(--accent)]/12 text-[17px] font-semibold text-[var(--accent)]">
          {(bot.display_name || bot.username || 'Q').slice(0, 1).toUpperCase()}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5">
            <span className="truncate text-[16px] font-medium">{bot.display_name || bot.username || bot.bot_id}</span>
            <Badge tone={running ? 'ok' : 'muted'}>{running ? '运行中' : bot.status === 'error' ? '异常' : '已停止'}</Badge>
          </div>
          <p className="mt-0.5 truncate text-[12px] font-mono text-[var(--text-3)]">{bot.bot_id}</p>
        </div>
        {busy ? (
          <Spinner />
        ) : (
          <button
            onClick={onToggle}
            disabled={!online}
            className={`h-8 rounded-full px-3 text-[13px] font-medium disabled:opacity-40 ${
              running ? 'bg-[var(--bg-sunken)] text-[var(--text)]' : 'bg-[var(--accent)] text-white'
            }`}
          >
            {running ? '停止' : '启动'}
          </button>
        )}
      </div>
      <div className="mt-2 flex items-center gap-3 pl-14">
        <button onClick={onRemove} className="text-[12px] text-[var(--danger)] active:opacity-60">
          移除
        </button>
        {bot.caps?.length ? (
          <span className="text-[11px] text-[var(--text-3)]">支持 {bot.caps.slice(0, 3).join(' · ')}</span>
        ) : null}
      </div>
    </div>
  )
}
