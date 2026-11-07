import type { ReactNode } from 'react'

type Span = { t: string; text?: string; url?: string }
type Block = { type: string; spans?: Span[]; text?: string; lang?: string }

type Doc = { v?: number; blocks?: Block[] }

// 渲染协议里的 body.doc，不理解的结构一律退回纯文本。
// 只有服务端声明 rich_text 能力时才走这里。
export function RichBody({ doc, plain }: { doc?: Doc; plain: string }) {
  const blocks = doc?.blocks
  if (!Array.isArray(blocks) || blocks.length === 0) return <>{plain}</>

  return (
    <div className="flex flex-col gap-2">
      {blocks.map((b, i) => {
        const content = (b.spans ?? []).map((s, j) => <span key={j}>{renderSpan(s)}</span>)
        switch (b.type) {
          case 'code':
            return (
              <pre
                key={i}
                className="overflow-x-auto rounded-xl bg-black/25 p-2.5 font-mono text-[13px] leading-relaxed text-inherit"
              >
                <code>{b.text ?? (b.spans ?? []).map((s) => s.text).join('')}</code>
              </pre>
            )
          case 'quote':
            return (
              <blockquote key={i} className="border-l-2 border-current/40 pl-3 text-[14px] opacity-90">
                {content}
              </blockquote>
            )
          default:
            return <p key={i}>{content}</p>
        }
      })}
    </div>
  )
}

function renderSpan(s: Span): ReactNode {
  switch (s.t) {
    case 'bold':
      return <strong>{s.text}</strong>
    case 'italic':
      return <em>{s.text}</em>
    case 'strike':
      return <s>{s.text}</s>
    case 'code':
      return <code className="rounded bg-black/20 px-1 py-0.5 font-mono text-[13px]">{s.text}</code>
    case 'link':
      return (
        <a href={s.url} target="_blank" rel="noreferrer noopener" className="underline underline-offset-2">
          {s.text || s.url}
        </a>
      )
    default:
      return <>{s.text}</>
  }
}
