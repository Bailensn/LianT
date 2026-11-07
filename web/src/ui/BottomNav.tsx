import { NavLink, useLocation } from 'react-router-dom'

type Item = { to: string; label: string; icon: (active: boolean) => React.ReactNode }

const stroke = (active: boolean) => (active ? 2.2 : 1.8)

const items: Item[] = [
  {
    to: '/chats',
    label: '会话',
    icon: (a) => (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={stroke(a)} strokeLinecap="round" strokeLinejoin="round">
        <path d="M21 11.5a8.5 8.5 0 0 1-8.5 8.5H8l-5 3 1.6-4.6A8.5 8.5 0 1 1 21 11.5Z" />
      </svg>
    ),
  },
  {
    to: '/bots',
    label: '机器人',
    icon: (a) => (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={stroke(a)} strokeLinecap="round" strokeLinejoin="round">
        <rect x="4" y="7" width="16" height="12" rx="4" />
        <path d="M12 3v4M8.5 12.5h.01M15.5 12.5h.01" />
      </svg>
    ),
  },
  {
    to: '/settings',
    label: '设置',
    icon: (a) => (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={stroke(a)} strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="3.2" />
        <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-2.9 1.2v.2a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-2.9-1.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1A1.7 1.7 0 0 0 3 15H2.9a2 2 0 1 1 0-4H3a1.7 1.7 0 0 0 1.2-2.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1A1.7 1.7 0 0 0 9.9 4.1V4a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 2.9 1.2l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1A1.7 1.7 0 0 0 21 11h.1a2 2 0 1 1 0 4H21Z" />
      </svg>
    ),
  },
]

export function BottomNav() {
  const { pathname } = useLocation()
  if (pathname.startsWith('/chat/') || pathname === '/pair') return null
  return (
    <div className="pointer-events-none fixed inset-x-0 bottom-0 z-30 flex justify-center pb-[max(0.75rem,env(safe-area-inset-bottom))]">
      <nav className="glass pointer-events-auto flex items-center gap-1 rounded-[26px] border border-[var(--sep)] p-1.5 shadow-[0_12px_40px_-12px_rgb(0_0_0/0.28)]">
        {items.map((it) => (
          <NavLink
            key={it.to}
            to={it.to}
            className={({ isActive }) =>
              `flex w-[92px] flex-col items-center gap-0.5 rounded-[20px] py-1.5 transition ${
                isActive ? 'bg-[color-mix(in_oklab,var(--accent)_14%,transparent)] text-[var(--accent)]' : 'text-[var(--text-2)]'
              }`
            }
          >
            {({ isActive }) => (
              <>
                {it.icon(isActive)}
                <span className="text-[10px] font-medium tracking-wide">{it.label}</span>
              </>
            )}
          </NavLink>
        ))}
      </nav>
    </div>
  )
}
