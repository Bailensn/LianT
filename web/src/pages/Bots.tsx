import { useState } from 'react'
import { useStore, type Bot } from '../store'
import { Page, Group, Divider, Field, Badge, Empty, Spinner, Sheet, Button } from '../ui/Common'

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
      // 服务端在 bot.add 里就会把机器人拉起来，这里只刷新列表。
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

      <Sheet
        open={adding}
        title="添加 QQ 机器人"
        onClose={() => setAdding(false)}
        footer={
          <Button full onClick={submit} disabled={busy || !appId.trim() || !secret.trim()}>
            {busy ? '验证中…' : '保存并启动'}
          </Button>
        }
      >
        <div className="mx-4 mb-3 flex gap-2 pt-1">
          <span className="rounded-full bg-[var(--accent)]/12 px-3 py-1.5 text-[13px] font-medium text-[var(--accent)]">
            QQ 机器人
          </span>
          <span className="rounded-full bg-[var(--bg-sunken)] px-3 py-1.5 text-[13px] text-[var(--text-3)]">
            Telegram · 即将支持
          </span>
        </div>
        <div className="card mx-4 mb-4 overflow-hidden">
          <Field label="AppID" value={appId} onChange={setAppId} placeholder="1020xxxxx" mono inputMode="numeric" />
          <Divider />
          <Field label="AppSecret" value={secret} onChange={setSecret} placeholder="平台密钥" type="password" mono />
          <Divider />
          <Field label="备注名 · 可留空" value={label} onChange={setLabel} placeholder="例如 客服号" />
        </div>
        {error && (
          <p className="mx-4 mb-3 rounded-2xl bg-[color-mix(in_oklab,var(--danger)_12%,transparent)] p-3 text-[13px] leading-relaxed text-[var(--danger)]">
            {error}
          </p>
        )}
        <p className="px-5 pb-4 text-[13px] leading-relaxed text-[var(--text-3)]">
          凭据在 QQ 开放平台「机器人 → 开发设置」里；保存时会实时校验并读取昵称。
          AppSecret 只存在服务端，加密入库，网页端不会再读出来。
        </p>
      </Sheet>
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
            className={`h-10 rounded-full px-4 text-[14px] font-medium disabled:opacity-40 ${
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
