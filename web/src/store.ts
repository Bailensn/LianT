import { create } from 'zustand'
import {
  clearIdentity,
  createKey,
  loadIdentity,
  parsePair,
  saveIdentity,
  type Identity,
} from './lib/identity'
import { LiantClient, pairDevice, selfRegister, uuid, type ConnState } from './lib/ws'

export type Bot = {
  bot_id: string
  platform: string
  username?: string
  display_name?: string
  status: string
  caps?: string[]
  constraints?: {
    can_initiate?: boolean
    reply_window_s?: number
    max_replies_per_inbound?: number
    requires_mention?: boolean
  }
  since?: number
}

export type Msg = {
  msg_id: string
  bot_id: string
  peer_id: string
  peer_kind: string
  sender_name?: string
  body: { plain: string; doc?: { blocks?: unknown[] } | null }
  msg_type?: string
  ts: number
  direction: 'in' | 'out'
  in_reply_to?: string
  pending?: boolean
  failed?: boolean
}

export type Device = {
  device_id: string
  name: string
  created_at: number
  last_seen_at: number
}

export type AuditEntry = {
  ts: number
  kind: string
  device_id?: string
  name?: string
  ip?: string
  reason?: string
}

export type Conversation = {
  key: string
  bot_id: string
  peer_id: string
  peer_kind: string
  title: string
  last?: Msg
  unread: number
}

type State = {
  identity: Identity | null
  state: ConnState
  detail: string
  error: string
  bots: Bot[]
  devices: Device[]
  audit: AuditEntry[]
  msgs: Msg[]
  read: Record<string, number>
  remarks: Record<string, string>
  titles: Record<string, string>
  peerHints: Record<string, string>
  caps: string[]
  booted: boolean
}

type Store = State & Actions

type Actions = {
  boot: () => Promise<void>
  reconnect: () => Promise<void>
  pair: (input: string, deviceName: string) => Promise<void>
  pairAuto: (deviceName: string) => Promise<void>
  unpair: () => Promise<void>
  refreshBots: () => Promise<void>
  addBot: (platform: string, credentials: Record<string, string>, label?: string) => Promise<string>
  startBot: (id: string) => Promise<void>
  botStatus: (id: string) => Promise<void>
  stopBot: (id: string) => Promise<void>
  removeBot: (id: string) => Promise<void>
  send: (bot: Bot, peerId: string, peerKind: string, text: string, inReplyTo?: string) => Promise<void>
  retrySend: (msgId: string) => Promise<void>
  history: (botId: string, peerId: string) => Promise<void>
  media: Record<string, string>
  fetchMedia: (botId: string, msgId: string) => Promise<void>
  loadDevices: () => Promise<void>
  revokeDevice: (id: string) => Promise<void>
  loadAudit: () => Promise<void>
  markRead: (key: string) => void
  setRemark: (key: string, name: string) => void
  clearError: () => void
}

let client: LiantClient | null = null
const KEYCLIENT = 'client'

export const convKey = (bot: string, peer: string) => `${bot}|${peer}`

export const useStore = create<State & Actions>((set, get) => ({
  identity: null,
  state: 'idle',
  detail: '',
  error: '',
  bots: [],
  devices: [],
  audit: [],
  msgs: [],
  media: {},
  read: {},
  remarks: {},
  titles: {},
  peerHints: {},
  caps: [],
  booted: false,

  async boot() {
    if (get().booted) return
    const identity = (await loadIdentity()) ?? null
    const read = await loadKV<Record<string, number>>('read', {})
    const remarks = await loadKV<Record<string, string>>('remarks', {})
    const titles = await loadKV<Record<string, string>>('titles', {})
    const peerHints = await loadKV<Record<string, string>>('peerHints', {})
    const msgs = (await loadMessages()).sort(byTime)
    set({ identity, read, remarks, titles, peerHints, msgs, booted: true })
    if (identity) await connect(identity, set, get)
  },

  async reconnect() {
    const identity = get().identity
    if (!identity) return
    set({ state: 'connecting', detail: '' })
    await connect(identity, set, get)
  },

  async pair(input, deviceName) {
    const p = parsePair(input)
    if (!p) throw new Error('这段内容不是有效的配对信息')
    if (p.exp && p.exp * 1000 < Date.now()) throw new Error('配对码已过期，请在服务端重新执行 pair')
    const { key, pub } = await createKey()
    const res = await pairDevice(p, deviceName || '网页端', pub, key)
    const identity: Identity = {
      deviceId: res.deviceId,
      serviceId: res.serviceId,
      serviceName: res.serviceName,
      host: p.host,
      port: p.port,
      fp: p.fp,
      key,
      pub,
    }
    await saveIdentity(identity)
    set({ identity, error: '' })
    await connect(identity, set, get)
  },

  async pairAuto(deviceName) {
    const { key, pub } = await createKey()
    const res = await selfRegister(deviceName || '网页端', pub)
    const port = Number(location.port || (location.protocol === 'https:' ? 443 : 80))
    const identity: Identity = {
      deviceId: res.deviceId,
      serviceId: res.serviceId,
      serviceName: res.serviceName,
      host: location.hostname,
      port,
      fp: res.serviceId,
      key,
      pub,
    }
    await saveIdentity(identity)
    set({ identity, error: '' })
    await connect(identity, set, get)
  },

  async unpair() {
    client?.stop()
    client = null
    await clearIdentity()
    set({ identity: null, state: 'idle', bots: [], msgs: [], detail: '' })
  },

  async refreshBots() {
    if (!client) return
    const res = await client.request<{ bots: Bot[] }>('bot.list', {})
    set({ bots: res.bots ?? [] })
  },

  async addBot(platform, credentials, label) {
    if (!client) throw new Error('尚未连接')
    const res = await client.request<{ bot_id: string }>('bot.add', { platform, credentials, label })
    await get().refreshBots()
    return res.bot_id
  },

  async botStatus(id) {
    if (!client) return
    const st = await client.request<{ status: string; last_error?: string }>('bot.status', { bot_id: id })
    set({ bots: get().bots.map((b) => (b.bot_id === id ? { ...b, status: st.status } : b)) })
  },

  async startBot(id) {
    await client?.request('bot.start', { bot_id: id })
    await get().botStatus(id)
    await get().refreshBots()
  },

  async stopBot(id) {
    await client?.request('bot.stop', { bot_id: id })
    await get().botStatus(id)
    await get().refreshBots()
  },

  async removeBot(id) {
    await client?.request('bot.remove', { bot_id: id })
    await get().refreshBots()
  },

  async send(bot, peerId, peerKind, text, inReplyTo) {
    if (!client) throw new Error('尚未连接')
    // 先乐观上屏，服务端确认后再换成真实 msg_id。
    const localId = `local-${uuid()}`
    const optimistic: Msg = {
      msg_id: localId,
      bot_id: bot.bot_id,
      peer_id: peerId,
      peer_kind: peerKind,
      body: { plain: text },
      ts: Date.now(),
      direction: 'out',
      in_reply_to: inReplyTo,
      pending: true,
    }
    set({ msgs: [...get().msgs, optimistic].sort(byTime) })

    try {
      const res = await client.request<{ msg_id: string; ts: number }>('msg.send', {
        bot_id: bot.bot_id,
        peer_id: peerId,
        body: { plain: text },
        in_reply_to: inReplyTo,
      })
      const confirmed: Msg = { ...optimistic, msg_id: res.msg_id, ts: res.ts || optimistic.ts, pending: false }
      set({
        msgs: get().msgs.map((m) => (m.msg_id === localId ? confirmed : m)).sort(byTime),
      })
      void saveMessage(confirmed)
    } catch (err) {
      set({
        msgs: get().msgs.map((m) => (m.msg_id === localId ? { ...m, pending: false, failed: true } : m)),
      })
      throw err
    }
  },

  async retrySend(msgId) {
    const msg = get().msgs.find((m) => m.msg_id === msgId)
    if (!msg || !msg.failed) return
    set({ msgs: get().msgs.filter((m) => m.msg_id !== msgId) })
    const bot = get().bots.find((b) => b.bot_id === msg.bot_id)
    if (!bot) return
    await get().send(bot, msg.peer_id, msg.peer_kind, msg.body.plain, msg.in_reply_to)
  },

  async fetchMedia(botId, msgId) {
    const key = `${botId}/${msgId}`
    if (get().media[key] || !client) return
    try {
      const res = await client.request<{ url: string }>('media.fetch', { bot_id: botId, msg_id: msgId })
      if (!res?.url) return
      const url = res.url.startsWith('http') ? res.url : `${location.origin}${res.url}`
      set({ media: { ...get().media, [key]: url } })
    } catch {
      set({ media: { ...get().media, [key]: '' } })
    }
  },

  async loadDevices() {
    if (!client) return
    const res = await client.request<{ devices: Device[] }>('device.list', {})
    set({ devices: res.devices ?? [] })
  },

  async revokeDevice(id) {
    await client?.request('device.revoke', { device_id: id })
    await get().loadDevices()
    await get().loadAudit()
  },

  async loadAudit() {
    if (!client) return
    const res = await client.request<{ entries: AuditEntry[] }>('audit.list', { limit: 20 })
    set({ audit: res.entries ?? [] })
  },

  async history(botId, peerId) {
    if (!client) return
    const res = await client.request<{ messages: Omit<Msg, 'direction'>[] }>('msg.list', {
      bot_id: botId,
      peer_id: peerId,
      limit: 100,
    })
    for (const m of res.messages ?? []) {
      append(set, get, { ...m, direction: 'in' })
    }
  },

  setRemark(key, name) {
    const remarks = { ...get().remarks }
    const trimmed = name.trim()
    if (trimmed) remarks[key] = trimmed
    else delete remarks[key]
    set({ remarks })
    void putKV('remarks', remarks)
  },

  markRead(key) {
    const read = { ...get().read, [key]: Date.now() }
    set({ read })
    void putKV('read', read)
  },

  clearError() {
    set({ error: '' })
  },
}))

// 同一毫秒内的消息按 msg_id 兜底排序，避免刷新后顺序抖动。
const byTime = (a: Msg, b: Msg) => a.ts - b.ts || String(a.msg_id).localeCompare(String(b.msg_id))

function append(set: (p: Partial<State>) => void, get: () => Store, msg: Msg) {
  if (get().msgs.some((m) => m.msg_id === msg.msg_id)) return
  const msgs = [...get().msgs, msg].sort(byTime)
  set({ msgs })
  void saveMessage(msg)
  if (msg.direction === 'in' && client) {
    void client.request('msg.ack', { msg_id: msg.msg_id }).catch(() => {})
  }
}

async function connect(
  identity: Identity,
  set: (p: Partial<State>) => void,
  get: () => Store,
) {
  client?.stop()
  client = new LiantClient(
    LiantClient.wsUrl,
    identity,
    {
      onState: (state, detail) => set({ state, detail: detail ?? '' }),
      onEvent: (env) => {
        if (env.type === 'auth.ok') {
          // 服务端换版本了说明刚重新部署，直接刷新一次拿到新前端。
          const payload = env.payload as { version?: string; caps?: string[] }
          if (payload.caps) set({ caps: payload.caps })
          const v = payload.version
          if (v) {
            const prev = localStorage.getItem('liant.server.version')
            localStorage.setItem('liant.server.version', v)
            if (prev && prev !== v) location.reload()
          }
          return
        }
        if (env.type === 'msg.inbound' || env.type === 'event.msg') {
          const p = env.payload as Omit<Msg, 'direction'>
          append(set, get, { ...p, direction: 'in' })
          return
        }
        if (env.type === 'event.bot') {
          // 服务端只推变化的那一台，按 bot_id 就地打补丁，不整表重拉。
          const p = env.payload as { bot_id: string; status: string; reason?: string }
          const bots = get().bots.map((b) => (b.bot_id === p.bot_id ? { ...b, status: p.status } : b))
          set({ bots })
          return
        }
        if (env.type === 'event.peer') {
          const ev = env.payload as { bot_id: string; peer_id: string; title?: string; hint?: string }
          const key = convKey(ev.bot_id, ev.peer_id)
          if (ev.title) {
            const titles = { ...get().titles, [key]: ev.title }
            set({ titles })
            void putKV('titles', titles)
          }
          if (ev.hint) {
            const peerHints = { ...get().peerHints, [key]: ev.hint }
            set({ peerHints })
            void putKV('peerHints', peerHints)
          }
          return
        }
        if (env.type === 'event.audit') {
          const entry = env.payload as AuditEntry
          set({ audit: [entry, ...get().audit].slice(0, 20) })
          if (entry.kind === 'device.revoke' || entry.kind === 'pair.ok') void get().loadDevices()
          return
        }
        if (env.type === 'error') {
          const e = env.payload as { message?: string }
          set({ error: e.message || '服务端返回错误' })
        }
      },
    },
  )
  try {
    await client.start()
    // 首屏一次拿全：bots + devices + audit，省掉三个来回。
    try {
      const snap = await client.request<{
        bots: Bot[]
        devices: Device[]
        audit: AuditEntry[]
      }>('state.snapshot', {})
      set({ bots: snap.bots ?? [], devices: snap.devices ?? [], audit: snap.audit ?? [] })
    } catch {
      // 老服务端没有这个方法，退回逐个拉取。
      await get().refreshBots()
      await get().loadDevices()
      await get().loadAudit()
    }
  } catch (err) {
    set({ state: 'error', detail: (err as Error).message })
  }
}

export function getClient(): LiantClient | null {
  return client
}

export { KEYCLIENT }

// ---------- 本地存储 ----------

function openDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open('liant', 1)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains('identity')) db.createObjectStore('identity')
      if (!db.objectStoreNames.contains('kv')) db.createObjectStore('kv')
      if (!db.objectStoreNames.contains('messages')) {
        const m = db.createObjectStore('messages', { keyPath: 'msg_id' })
        m.createIndex('by_bot', 'bot_id')
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

async function loadKV<T>(key: string, fallback: T): Promise<T> {
  const db = await openDB()
  return new Promise((resolve) => {
    const req = db.transaction('kv', 'readonly').objectStore('kv').get(key)
    req.onsuccess = () => resolve((req.result as T) ?? fallback)
    req.onerror = () => resolve(fallback)
  })
}

async function putKV(key: string, value: unknown): Promise<void> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction('kv', 'readwrite')
    tx.objectStore('kv').put(value, key)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}

async function saveMessage(msg: Msg): Promise<void> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction('messages', 'readwrite')
    tx.objectStore('messages').put(msg)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}

export async function loadMessages(): Promise<Msg[]> {
  const db = await openDB()
  return new Promise((resolve) => {
    const req = db.transaction('messages', 'readonly').objectStore('messages').getAll()
    req.onsuccess = () => resolve((req.result as Msg[]) ?? [])
    req.onerror = () => resolve([])
  })
}

export function deriveConversations(
  msgs: Msg[],
  read: Record<string, number>,
  remarks: Record<string, string> = {},
  titles: Record<string, string> = {},
): Conversation[] {
  const map = new Map<string, Conversation>()
  for (const m of msgs) {
    const key = convKey(m.bot_id, m.peer_id)
    const conv = map.get(key) ?? {
      key,
      bot_id: m.bot_id,
      peer_id: m.peer_id,
      peer_kind: m.peer_kind,
      title: remarks[key] || titles[key] || m.sender_name || m.peer_id,
      unread: 0,
      last: undefined,
    }
    if (!conv.last || m.ts >= conv.last.ts) conv.last = m
    conv.title = remarks[key] || titles[key] || m.sender_name || conv.title
    if (m.direction === 'in' && m.ts > (read[key] ?? 0)) conv.unread++
    map.set(key, conv)
  }
  // 群聊标题优先用 sender_name，没有就用 peer_id。
  return [...map.values()].sort((a, b) => (b.last?.ts ?? 0) - (a.last?.ts ?? 0))
}
