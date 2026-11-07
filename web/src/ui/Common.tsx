import { useEffect, type ReactNode } from 'react'
import { Link } from 'react-router-dom'

export function Page({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex h-full flex-col">
      <header className="mx-auto flex w-full max-w-[780px] items-end justify-between px-5 pt-[max(1rem,env(safe-area-inset-top))] pb-2 md:pt-8">
        <h1 className="text-[34px] leading-[1.1] font-bold tracking-tight">{title}</h1>
        {action}
      </header>
      <div className="mx-auto w-full max-w-[780px] flex-1 overflow-y-auto overscroll-contain px-4 pb-40 no-scrollbar md:pb-10">
        {children}
      </div>
    </div>
  )
}

export function Group({ title, children, footer }: { title?: string; footer?: string; children: ReactNode }) {
  return (
    <section className="mb-7">
      {title && <h2 className="px-4 pb-2 text-[13px] font-medium tracking-wide text-[var(--text-2)]">{title}</h2>}
      <div className="card overflow-hidden">{children}</div>
      {footer && <p className="px-4 pt-2 text-[13px] leading-snug text-[var(--text-3)]">{footer}</p>}
    </section>
  )
}

export function Row({
  leading,
  title,
  subtitle,
  trailing,
  onClick,
  to,
  danger,
}: {
  leading?: ReactNode
  title: ReactNode
  subtitle?: ReactNode
  trailing?: ReactNode
  onClick?: () => void
  to?: string
  danger?: boolean
}) {
  const body = (
    <div className="flex min-h-[48px] items-center gap-3 px-4 py-3 transition active:bg-[var(--bg-sunken)] md:hover:bg-[var(--bg-sunken)]">
      {leading}
      <div className="min-w-0 flex-1">
        <div className={`truncate text-[16px] ${danger ? 'text-[var(--danger)]' : ''}`}>{title}</div>
        {subtitle && <div className="mt-0.5 truncate text-[13px] text-[var(--text-2)]">{subtitle}</div>}
      </div>
      {trailing}
    </div>
  )
  if (to) return <Link to={to}>{body}</Link>
  if (onClick) return <button onClick={onClick} className="block w-full text-left">{body}</button>
  return body
}

export function Divider() {
  return <div className="ml-4 h-px bg-[var(--sep)]" />
}

export function Button({
  children,
  onClick,
  variant = 'primary',
  disabled,
  full,
}: {
  children: ReactNode
  onClick?: () => void
  variant?: 'primary' | 'ghost' | 'danger'
  disabled?: boolean
  full?: boolean
}) {
  const base =
    'inline-flex h-12 min-h-[48px] items-center justify-center rounded-2xl px-5 text-[17px] font-medium transition active:scale-[0.97] disabled:opacity-40'
  const look =
    variant === 'primary'
      ? 'bg-[var(--accent)] text-white'
      : variant === 'danger'
        ? 'bg-[color-mix(in_oklab,var(--danger)_14%,transparent)] text-[var(--danger)]'
        : 'bg-[var(--bg-sunken)] text-[var(--text)]'
  return (
    <button className={`${base} ${look} ${full ? 'w-full' : ''}`} onClick={onClick} disabled={disabled}>
      {children}
    </button>
  )
}

export function Field({
  label,
  value,
  onChange,
  placeholder,
  type = 'text',
  mono,
  inputMode,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  placeholder?: string
  type?: string
  mono?: boolean
  inputMode?: 'text' | 'numeric' | 'tel' | 'email'
}) {
  return (
    <label className="block px-4 py-3">
      <span className="text-[13px] text-[var(--text-2)]">{label}</span>
      <input
        className={`mt-1 w-full bg-transparent text-[16px] outline-none placeholder:text-[var(--text-3)] ${mono ? 'font-mono text-[14px]' : ''}`}
        value={value}
        type={type}
        inputMode={type === 'number' ? 'numeric' : inputMode}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
      />
    </label>
  )
}

// 底部抽屉：拖拽条 + 60% 高度 + 锁死底层滚动；表单类不允许点遮罩静默关闭。
export function Sheet({
  open,
  title,
  onClose,
  children,
  footer,
  dismissable = false,
}: {
  open: boolean
  title: string
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  dismissable?: boolean
}) {
  useEffect(() => {
    if (!open) return
    document.body.classList.add('sheet-open')
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && dismissable) onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => {
      document.body.classList.remove('sheet-open')
      window.removeEventListener('keydown', onKey)
    }
  }, [open, dismissable, onClose])

  if (!open) return null
  return (
    <div
      className="fixed inset-0 z-40 flex items-end bg-black/40"
      onClick={() => dismissable && onClose()}
    >
      <div
        className="flex max-h-[85vh] w-full flex-col rounded-t-[28px] bg-[var(--bg)] pb-[max(16px,env(safe-area-inset-bottom))]"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mx-auto mt-2 h-1 w-9 shrink-0 rounded-full bg-[var(--text-3)]/40" />
        <div className="flex shrink-0 items-center justify-between px-5 py-3">
          <button onClick={onClose} className="min-h-[44px] text-[17px] text-[var(--text-2)]">
            取消
          </button>
          <p className="text-[17px] font-semibold">{title}</p>
          <div className="w-[44px]" />
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
        {footer && <div className="shrink-0 px-5 pt-2">{footer}</div>}
      </div>
    </div>
  )
}

export function Empty({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="flex flex-col items-center justify-center px-8 py-24 text-center">
      <div className="mb-3 h-12 w-12 rounded-2xl bg-[var(--bg-sunken)]" />
      <p className="text-[16px] font-medium">{title}</p>
      {hint && <p className="mt-1 max-w-[280px] text-[13px] leading-relaxed text-[var(--text-2)]">{hint}</p>}
    </div>
  )
}

export function Badge({ children, tone = 'muted' }: { children: ReactNode; tone?: 'ok' | 'warn' | 'muted' }) {
  const color = tone === 'ok' ? 'var(--ok)' : tone === 'warn' ? 'var(--danger)' : 'var(--text-2)'
  return (
    <span
      className="rounded-full px-2 py-0.5 text-[12px] font-medium"
      style={{ color, background: `color-mix(in oklab, ${color} 14%, transparent)` }}
    >
      {children}
    </span>
  )
}

export function Spinner() {
  return (
    <span className="inline-block h-4 w-4 animate-spin rounded-full border-2 border-[var(--text-3)] border-t-transparent" />
  )
}

export function formatTime(ts: number): string {
  const d = new Date(ts)
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  if (sameDay) return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleDateString('zh-CN', { month: 'numeric', day: 'numeric' })
}
