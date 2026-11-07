import { useEffect, useState } from 'react'
import { useStore } from '../store'
import { createKey, parsePair } from '../lib/identity'

export default function Pair() {
  const pair = useStore((s) => s.pair)
  const pairAuto = useStore((s) => s.pairAuto)
  const [autoBusy, setAutoBusy] = useState(false)
  const [text, setText] = useState('')
  const [name, setName] = useState('网页端')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [parsed, setParsed] = useState<string | null>(null)

  useEffect(() => {
    // 支持从 `/#pair=…` 或 `?pair=…` 直接带配对信息打开。
    const m = location.hash.match(/pair=([^&?]+)/)
    if (m) setText(decodeURIComponent(m[1]))
  }, [])

  useEffect(() => {
    const p = parsePair(text)
    setParsed(p ? `${p.host}:${p.port}` : null)
  }, [text])

  async function submit() {
    setBusy(true)
    setError('')
    try {
      await pair(text, name)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="mx-auto flex h-full max-w-[520px] flex-col px-5 pt-[max(2rem,env(safe-area-inset-top))] pb-8">
      <div className="mb-8">
        <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-[18px] bg-[var(--accent)] text-[26px] font-semibold text-white">
          L
        </div>
        <h1 className="text-[32px] leading-tight font-bold tracking-tight">连接 LianT</h1>
        <p className="mt-2 text-[15px] leading-relaxed text-[var(--text-2)]">
          在服务器上执行 <code className="rounded bg-[var(--bg-sunken)] px-1.5 py-0.5 font-mono text-[13px]">LianT pair</code>
          ，把终端里的配对信息粘贴到这里。
        </p>
      </div>

      <button
        onClick={async () => {
          setAutoBusy(true)
          setError('')
          try {
            await pairAuto(name)
          } catch (err) {
            setError((err as Error).message)
          } finally {
            setAutoBusy(false)
          }
        }}
        disabled={autoBusy}
        className="h-12 w-full rounded-2xl bg-[var(--accent)] text-[17px] font-semibold text-white transition active:scale-[0.98] disabled:opacity-40"
      >
        {autoBusy ? '正在登记…' : '直接进入'}
      </button>
      <p className="mt-2 mb-6 px-1 text-[12px] leading-relaxed text-[var(--text-3)]">
        服务端开启了登录认证时，不必再输配对码：这台浏览器会直接登记成一个设备。
      </p>

      <div className="mb-4 flex items-center gap-3">
        <div className="h-px flex-1 bg-[var(--sep)]" />
        <span className="text-[12px] text-[var(--text-3)]">或者用配对码</span>
        <div className="h-px flex-1 bg-[var(--sep)]" />
      </div>

      <div className="card mb-4 overflow-hidden">
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          rows={5}
          spellCheck={false}
          placeholder='{"v":1,"host":"192.168.1.10","port":8443,...}'
          className="w-full resize-none bg-transparent px-4 py-4 font-mono text-[13px] leading-relaxed outline-none placeholder:text-[var(--text-3)]"
        />
        <div className="flex items-center justify-between border-t border-[var(--sep)] px-4 py-2">
          <span className="text-[13px] text-[var(--text-2)]">{parsed ? `服务地址 ${parsed}` : '等待配对信息'}</span>
          <label className="cursor-pointer text-[13px] font-medium text-[var(--accent)]">
            从相册识别
            <input
              type="file"
              accept="image/*"
              className="hidden"
              onChange={async (e) => {
                const file = e.target.files?.[0]
                if (!file) return
                try {
                  const { decodeQr } = await import('../lib/qr')
                  setText(await decodeQr(file))
                } catch (err) {
                  setError((err as Error).message)
                }
              }}
            />
          </label>
        </div>
      </div>

      <div className="card mb-6 overflow-hidden">
        <label className="flex items-center justify-between px-4 py-3">
          <span className="text-[16px]">设备名称</span>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-40 bg-transparent text-right text-[16px] outline-none"
          />
        </label>
      </div>

      {error && <p className="mb-3 px-1 text-[13px] leading-relaxed text-[var(--danger)]">{error}</p>}

      <button
        onClick={submit}
        disabled={!parsed || busy}
        className="h-11 w-full rounded-2xl bg-[var(--bg-sunken)] text-[16px] font-medium text-[var(--text)] transition active:scale-[0.98] disabled:opacity-40"
      >
        {busy ? '正在配对…' : '用配对码连接'}
      </button>

      <p className="mt-4 px-1 text-[12px] leading-relaxed text-[var(--text-3)]">
        配对码 60 秒内有效，且服务端终端会要求本地确认。页面必须通过 HTTPS 或 localhost 打开，否则浏览器不允许生成设备密钥。
      </p>
      <KeyProbe />
    </div>
  )
}

// 浏览器只在安全上下文里开放 Web Crypto；提前打招呼比报错友好。
function KeyProbe() {
  const [ok, setOk] = useState(true)
  useEffect(() => {
    void createKey()
      .then(() => setOk(true))
      .catch(() => setOk(false))
  }, [])
  if (ok) return null
  return (
    <p className="mt-4 rounded-2xl bg-[color-mix(in_oklab,var(--danger)_12%,transparent)] p-3 text-[12px] leading-relaxed text-[var(--danger)]">
      当前页面不是安全上下文，无法生成设备密钥。请用 https:// 或本机地址访问。
    </p>
  )
}
