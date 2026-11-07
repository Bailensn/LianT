import { NavLink } from 'react-router-dom'

const items = [
  {
    to: '/chats',
    label: '会话',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <path d="M21 11.5a8.5 8.5 0 0 1-8.5 8.5H8l-5 3 1.6-4.6A8.5 8.5 0 1 1 21 11.5Z" />
      </svg>
    ),
  },
  {
    to: '/bots',
    label: '机器人',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="4" y="7" width="16" height="12" rx="4" />
        <path d="M12 3v4M8.5 12.5h.01M15.5 12.5h.01" />
      </svg>
    ),
  },
  {
    to: '/settings',
    label: '设置',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="3.2" />
        <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-2.9 1.2v.2a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-2.9-1.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1A1.7 1.7 0 0 0 3 15H2.9a2 2 0 1 1 0-4H3a1.7 1.7 0 0 0 1.2-2.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1A1.7 1.7 0 0 0 9.9 4.1V4a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 2.9 1.2l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1A1.7 1.7 0 0 0 21 11h.1a2 2 0 1 1 0 4H21Z" />
      </svg>
    ),
  },
]

// 桌面端的左侧导航：手机宽度下隐藏，由底部胶囊接管。
export function DesktopSidebar() {
  return (
    <aside className="hidden w-[228px] shrink-0 flex-col border-r border-[var(--sep)] bg-[var(--bg-sunken)]/40 px-3 py-6 md:flex">
      <div className="mb-8 flex items-center gap-2.5 px-2">
        <div className="flex h-9 w-9 items-center justify-center rounded-[11px] bg-[var(--accent)] text-[17px] font-semibold text-white">
          L
        </div>
        <span className="text-[17px] font-semibold tracking-tight">LianT</span>
      </div>
      <nav className="flex flex-col gap-1">
        {items.map((it) => (
          <NavLink
            key={it.to}
            to={it.to}
            className={({ isActive }) =>
              `flex items-center gap-3 rounded-xl px-3 py-2.5 text-[14px] transition ${
                isActive
                  ? 'bg-[var(--bg-elevated)] font-medium text-[var(--text)] shadow-[0_1px_2px_rgb(0_0_0/0.06)]'
                  : 'text-[var(--text-2)] hover:bg-[var(--bg-elevated)]/70 hover:text-[var(--text)]'
              }`
            }
          >
            {it.icon}
            {it.label}
          </NavLink>
        ))}
      </nav>
    </aside>
  )
}
