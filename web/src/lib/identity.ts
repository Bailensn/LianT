// 设备身份：私钥由 Web Crypto 生成，不可导出，保存在 IndexedDB 里。
// 服务端要求 manager_pub 是 base64 的 DER SubjectPublicKeyInfo，
// 签名是 base64 的 ASN.1 DER（Go 的 ecdsa.VerifyASN1）。

const DB_NAME = 'liant'
const STORE = 'identity'
const KEY_ID = 'self'

export type Identity = {
  deviceId: string
  serviceId: string
  serviceName: string
  host: string
  port: number
  fp: string
  key: CryptoKey
  pub: string
}

function openDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, 1)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains(STORE)) db.createObjectStore(STORE)
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

async function idbGet<T>(store: string, key: IDBValidKey): Promise<T | undefined> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(store, 'readonly')
    const req = tx.objectStore(store).get(key)
    req.onsuccess = () => resolve(req.result as T | undefined)
    req.onerror = () => reject(req.error)
  })
}

async function idbPut(store: string, value: unknown, key?: IDBValidKey): Promise<void> {
  const db = await openDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(store, 'readwrite')
    if (key === undefined) tx.objectStore(store).put(value)
    else tx.objectStore(store).put(value, key)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}

export const loadIdentity = () => idbGet<Identity>(STORE, KEY_ID)
export const saveIdentity = (id: Identity) => idbPut(STORE, id, KEY_ID)

export async function clearIdentity(): Promise<void> {
  const db = await openDB()
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE, 'readwrite')
    tx.objectStore(STORE).clear()
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}

function toB64(bytes: ArrayBuffer | Uint8Array): string {
  const view = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes)
  let out = ''
  for (const b of view) out += String.fromCharCode(b)
  return btoa(out)
}

function fromB64(s: string): Uint8Array {
  const raw = atob(s)
  const out = new Uint8Array(raw.length)
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i)
  return out
}

// Web Crypto 的 ECDSA 签名是 IEEE P1363（r||s），Go 侧要 ASN.1 DER。
function p1363ToDer(raw: Uint8Array): Uint8Array {
  const trim = (b: Uint8Array) => {
    let i = 0
    while (i < b.length - 1 && b[i] === 0) i++
    let out = b.slice(i)
    if (out[0] & 0x80) out = new Uint8Array([0, ...out])
    return out
  }
  const half = raw.length / 2
  const r = trim(raw.slice(0, half))
  const s = trim(raw.slice(half))
  const len = 2 + r.length + 2 + s.length
  const der = new Uint8Array(2 + len)
  der[0] = 0x30
  der[1] = len
  der[2] = 0x02
  der[3] = r.length
  der.set(r, 4)
  der[4 + r.length] = 0x02
  der[5 + r.length] = s.length
  der.set(s, 6 + r.length)
  return der
}

export async function createKey(): Promise<{ key: CryptoKey; pub: string }> {
  const pair = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, true, [
    'sign',
    'verify',
  ])
  const spki = await crypto.subtle.exportKey('spki', pair.publicKey)
  const pkcs8 = await crypto.subtle.exportKey('pkcs8', pair.privateKey)
  // 私钥转成不可导出后再落库，浏览器再也不能把它读出来。
  const key = await crypto.subtle.importKey(
    'pkcs8',
    pkcs8 as ArrayBuffer,
    { name: 'ECDSA', namedCurve: 'P-256' },
    false,
    ['sign'],
  )
  return { key, pub: toB64(spki) }
}

export async function sign(key: CryptoKey, message: Uint8Array): Promise<string> {
  const raw = new Uint8Array(await crypto.subtle.sign({ name: 'ECDSA', hash: 'SHA-256' }, key, message as BufferSource))
  return toB64(p1363ToDer(raw))
}

// DER（ASN.1）签名转回 IEEE P1363，Web Crypto 只认后者。
function derToP1363(der: Uint8Array): Uint8Array {
  let i = 0
  if (der[i++] !== 0x30) throw new Error('签名格式不正确')
  if (der[i] & 0x80) i += 1 + (der[i] & 0x7f)
  else i += 1
  const readInt = () => {
    if (der[i++] !== 0x02) throw new Error('签名格式不正确')
    const len = der[i++]
    let body = der.slice(i, i + len)
    i += len
    while (body.length > 32 && body[0] === 0) body = body.slice(1)
    const out = new Uint8Array(32)
    out.set(body, 32 - body.length)
    return out
  }
  const r = readInt()
  const s = readInt()
  const out = new Uint8Array(64)
  out.set(r, 0)
  out.set(s, 32)
  return out
}

// RFC 4648 base32（无填充、大写、每 4 位一组加连字符），与服务端指纹一致。
function base32Fingerprint(bytes: Uint8Array): string {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'
  let bits = 0
  let value = 0
  let out = ''
  for (const b of bytes) {
    value = (value << 8) | b
    bits += 8
    while (bits >= 5) {
      out += alphabet[(value >>> (bits - 5)) & 31]
      bits -= 5
    }
  }
  if (bits > 0) out += alphabet[(value << (5 - bits)) & 31]
  return out.match(/.{1,4}/g)?.join('-') ?? out
}

export async function fingerprintOf(spkiB64: string): Promise<string> {
  const der = fromB64(spkiB64)
  const digest = await crypto.subtle.digest('SHA-256', der as BufferSource)
  return base32Fingerprint(new Uint8Array(digest))
}

// 校验服务端身份：公钥指纹必须等于配对时钉住的那一个，且签名有效。
export async function verifyServiceProof(
  spkiB64: string,
  expectedFingerprint: string,
  message: Uint8Array,
  sigB64: string,
): Promise<void> {
  const fp = await fingerprintOf(spkiB64)
  if (expectedFingerprint && fp !== expectedFingerprint) {
    throw new Error('服务端身份指纹不匹配，可能有人在中间人攻击')
  }
  const key = await crypto.subtle.importKey(
    'spki',
    fromB64(spkiB64) as BufferSource,
    { name: 'ECDSA', namedCurve: 'P-256' },
    false,
    ['verify'],
  )
  const ok = await crypto.subtle.verify(
    { name: 'ECDSA', hash: 'SHA-256' },
    key,
    derToP1363(fromB64(sigB64)) as BufferSource,
    message as BufferSource,
  )
  if (!ok) throw new Error('服务端身份签名无效')
}

export type PairPayload = {
  v: number
  host: string
  port: number
  fp: string
  ticket: string
  nonce: string
  exp: number
}

// 解析二维码/链接里的配对信息，兼容裸 JSON、base64url 与 #pair= 片段。
export function parsePair(text: string): PairPayload | null {
  const raw = text.trim()
  if (!raw) return null
  const candidates: string[] = [raw]
  const m = raw.match(/[#?]pair=([^&\s]+)/)
  if (m) candidates.push(m[1])
  for (const c of candidates) {
    try {
      const json = c.trim().startsWith('{') ? c : new TextDecoder().decode(fromB64(c.replace(/-/g, '+').replace(/_/g, '/')))
      const obj = JSON.parse(json)
      if (obj && typeof obj.ticket === 'string' && typeof obj.host === 'string') {
        return {
          v: Number(obj.v ?? 1),
          host: String(obj.host),
          port: Number(obj.port ?? 8443),
          fp: String(obj.fp ?? ''),
          ticket: String(obj.ticket),
          nonce: String(obj.nonce ?? ''),
          exp: Number(obj.exp ?? 0),
        }
      }
    } catch {
      // 继续尝试下一种编码
    }
  }
  return null
}

export { fromB64, toB64 }
