// WSS 客户端：信封 JSON、签名握手、请求-响应关联、心跳与自动重连。
import { sign, verifyServiceProof, type Identity, type PairPayload } from './identity'

export type Envelope = {
  v: number
  id: string
  type: string
  ts: number
  reply_to?: string
  payload?: unknown
}

export type ConnState = 'idle' | 'connecting' | 'online' | 'reconnecting' | 'error'

type Handlers = {
  onState: (state: ConnState, detail?: string) => void
  onEvent: (env: Envelope) => void
}

const PING_MS = 30_000
const REQUEST_MS = 20_000

export class LiantClient {
  private ws: WebSocket | null = null
  private pending = new Map<string, { resolve: (v: Envelope) => void; reject: (e: Error) => void; timer: number }>()
  private pingTimer: number | null = null
  private retryTimer: number | null = null
  private retry = 0
  private closed = true

  private url: (host: string, port: number) => string
  private identity: Identity
  private handlers: Handlers

  constructor(url: (host: string, port: number) => string, identity: Identity, handlers: Handlers) {
    this.url = url
    this.identity = identity
    this.handlers = handlers
  }

  static wsUrl(host: string, port: number): string {
    const scheme = location.protocol === 'https:' ? 'wss' : 'ws'
    // 页面由 Service 自己托管时，同源最省事；否则用配对信息里的 host。
    const same = location.hostname === host && Number(location.port || (location.protocol === 'https:' ? 443 : 80)) === port
    const h = same ? location.host : `${host}:${port}`
    return `${scheme}://${h}/v1/ws`
  }

  async start(): Promise<void> {
    this.closed = false
    await this.open()
  }

  stop(): void {
    this.closed = true
    this.clearTimers()
    this.ws?.close()
    this.ws = null
    this.failAll(new Error('连接已关闭'))
    this.handlers.onState('idle')
  }

  private clearTimers() {
    if (this.pingTimer !== null) window.clearInterval(this.pingTimer)
    if (this.retryTimer !== null) window.clearTimeout(this.retryTimer)
    this.pingTimer = null
    this.retryTimer = null
  }

  private failAll(err: Error) {
    for (const [, p] of this.pending) {
      window.clearTimeout(p.timer)
      p.reject(err)
    }
    this.pending.clear()
  }

  private open(): Promise<void> {
    return new Promise((resolve, reject) => {
      const { host, port } = this.identity
      this.handlers.onState(this.retry > 0 ? 'reconnecting' : 'connecting')
      let settled = false
      const ws = new WebSocket(this.url(host, port))
      this.ws = ws

      ws.onopen = () => {
        // 服务端连上就先发 auth.challenge，我们在这里等它。
      }
      ws.onmessage = (ev) => {
        let env: Envelope
        try {
          env = JSON.parse(String(ev.data))
        } catch {
          return
        }
        if (env.type === 'auth.challenge') {
          void this.answerChallenge(env, ws)
            .then(() => {
              settled = true
              this.retry = 0
              this.handlers.onState('online')
              this.startPing()
              resolve()
            })
            .catch((err: Error) => {
              if (!settled) reject(err)
              ws.close()
            })
          return
        }
        this.dispatch(env)
      }
      ws.onerror = () => {
        if (!settled) reject(new Error('无法连接到服务'))
      }
      ws.onclose = () => {
        this.clearTimers()
        if (!settled) reject(new Error('连接被关闭'))
        if (this.closed) return
        this.scheduleRetry()
      }
    })
  }

  private async answerChallenge(env: Envelope, ws: WebSocket) {
    const ch = env.payload as {
      nonce: string
      ts: number
      service_pub?: string
      service_sig?: string
    }
    // 待签字节固定为 nonce \n 指纹 \n 十进制 ts。
    const msg = new TextEncoder().encode(`${ch.nonce}\n${this.identity.serviceId}\n${ch.ts}`)
    // 浏览器做不了证书钉扎，这里用服务端签名补回来：公钥指纹必须等于配对时钉住的值。
    if (ch.service_pub && ch.service_sig) {
      await verifyServiceProof(ch.service_pub, this.identity.serviceId, msg, ch.service_sig)
    } else if (this.identity.serviceId) {
      throw new Error('服务端没有提供身份证明，拒绝连接')
    }
    const sig = await sign(this.identity.key, msg)
    this.sendOn(ws, 'auth.response', { manager_id: this.identity.deviceId, sig })
  }

  private scheduleRetry() {
    if (this.closed) return
    this.retry++
    const delay = Math.min(1000 * 2 ** (this.retry - 1), 15_000)
    this.handlers.onState('reconnecting', `${Math.round(delay / 1000)} 秒后重试`)
    this.retryTimer = window.setTimeout(() => {
      this.open().catch(() => {
        /* onclose 会再排一次 */
      })
    }, delay)
  }

  private startPing() {
    if (this.pingTimer !== null) window.clearInterval(this.pingTimer)
    this.pingTimer = window.setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) this.send('ping', {})
    }, PING_MS)
  }

  private sendOn(ws: WebSocket, type: string, payload: unknown) {
    const env: Envelope = { v: 1, id: uuid(), type, ts: Date.now(), payload }
    ws.send(JSON.stringify(env))
    return env.id
  }

  send(type: string, payload: unknown): string {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) throw new Error('连接未就绪')
    return this.sendOn(this.ws, type, payload)
  }

  request<T = unknown>(type: string, payload: unknown): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      let id: string
      try {
        id = this.send(type, payload)
      } catch (err) {
        reject(err as Error)
        return
      }
      const timer = window.setTimeout(() => {
        this.pending.delete(id)
        reject(new Error('请求超时'))
      }, REQUEST_MS)
      this.pending.set(id, {
        resolve: (env) => resolve(env.payload as T),
        reject,
        timer,
      })
    })
  }

  private dispatch(env: Envelope) {
    if (env.reply_to) {
      const p = this.pending.get(env.reply_to)
      if (p) {
        this.pending.delete(env.reply_to)
        window.clearTimeout(p.timer)
        if (env.type === 'error') {
          const e = env.payload as { message?: string; code?: string }
          p.reject(new Error(e.message || e.code || '请求失败'))
        } else {
          p.resolve(env)
        }
        return
      }
    }
    if (env.type === 'pong' || env.type === 'ping') return
    this.handlers.onEvent(env)
  }
}

export function uuid(): string {
  if (crypto.randomUUID) return crypto.randomUUID()
  return Array.from(crypto.getRandomValues(new Uint8Array(16)), (b) => b.toString(16).padStart(2, '0')).join('')
}

// 已登录 Basic 认证时的自助登记：不需要配对码，直接登记这台浏览器。
export async function selfRegister(
  name: string,
  pub: string,
): Promise<{ deviceId: string; serviceId: string; serviceName: string }> {
  const scheme = location.protocol === 'https:' ? 'wss' : 'ws'
  const ws = new WebSocket(`${scheme}://${location.host}/v1/ws`)
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => {
      ws.close()
      reject(new Error('登记超时'))
    }, 15_000)
    ws.onopen = () => {
      ws.send(JSON.stringify({
        v: 1,
        id: uuid(),
        type: 'pair.request',
        ts: Date.now(),
        payload: { ticket: '', manager_name: name, manager_pub: pub, sig: '' },
      }))
    }
    ws.onmessage = (ev) => {
      let env: Envelope
      try {
        env = JSON.parse(String(ev.data))
      } catch {
        return
      }
      if (env.type === 'pair.ok') {
        const ok = env.payload as { device_id: string; service_id: string; service_name: string }
        window.clearTimeout(timer)
        ws.close()
        resolve({ deviceId: ok.device_id, serviceId: ok.service_id, serviceName: ok.service_name })
      } else if (env.type === 'pair.denied') {
        const d = env.payload as { reason: string }
        window.clearTimeout(timer)
        ws.close()
        reject(new Error(d.reason === 'no_ticket' ? '服务端没有开启免配对码接入' : reasonText(d.reason)))
      }
    }
    ws.onerror = () => {
      window.clearTimeout(timer)
      reject(new Error('无法连接到服务'))
    }
  })
}

// 一次性配对：连上后发 pair.request，成功返回 device_id。
export async function pairDevice(
  p: PairPayload,
  managerName: string,
  pub: string,
  key: CryptoKey,
): Promise<{ deviceId: string; serviceId: string; serviceName: string }> {
  const url = LiantClient.wsUrl(p.host, p.port)
  const ws = new WebSocket(url)
  const sig = await sign(key, new TextEncoder().encode(p.nonce))

  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => {
      ws.close()
      reject(new Error('配对超时，请确认服务端仍在等待'))
    }, 90_000)

    ws.onopen = () => {
      const env: Envelope = {
        v: 1,
        id: uuid(),
        type: 'pair.request',
        ts: Date.now(),
        payload: { ticket: p.ticket, manager_name: managerName, manager_pub: pub, sig },
      }
      ws.send(JSON.stringify(env))
    }
    ws.onmessage = (ev) => {
      let env: Envelope
      try {
        env = JSON.parse(String(ev.data))
      } catch {
        return
      }
      if (env.type === 'pair.ok') {
        const ok = env.payload as { device_id: string; service_id: string; service_name: string }
        window.clearTimeout(timer)
        ws.close()
        resolve({ deviceId: ok.device_id, serviceId: ok.service_id, serviceName: ok.service_name })
      } else if (env.type === 'pair.denied') {
        const d = env.payload as { reason: string }
        window.clearTimeout(timer)
        ws.close()
        reject(new Error(reasonText(d.reason)))
      } else if (env.type === 'error') {
        const e = env.payload as { message?: string }
        window.clearTimeout(timer)
        ws.close()
        reject(new Error(e.message || '配对失败'))
      }
    }
    ws.onerror = () => {
      window.clearTimeout(timer)
      reject(new Error('无法连接到服务，可能是证书未被信任'))
    }
    ws.onclose = () => {
      window.clearTimeout(timer)
    }
  })
}

function reasonText(reason: string): string {
  switch (reason) {
    case 'no_ticket':
      return '服务端没有待处理的配对码'
    case 'expired':
      return '配对码已过期'
    case 'used':
      return '配对码已被使用'
    case 'mismatch':
      return '配对码或 nonce 不匹配'
    case 'devices_full':
      return '设备数量已达上限'
    case 'denied':
      return '服务端拒绝了这次配对'
    case 'invalid_pub':
      return '公钥格式不被接受'
    case 'invalid_sig':
      return '签名校验失败'
    case 'name_not_allowed':
      return '这台设备的名称不在服务端白名单里'
    default:
      return `配对失败：${reason}`
  }
}
