import { useEffect } from 'react'
import { useStore } from '../store'
import { Page, Group, Row, Divider, Badge } from '../ui/Common'

function fmtTime(ts: number): string {
  if (!ts) return '从未'
  const d = new Date(ts)
  return d.toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

const kindText: Record<string, string> = {
  'auth.ok': '登录成功',
  'auth.fail': '登录失败',
  'pair.ok': '配对成功',
  'pair.denied': '配对被拒',
  'device.revoke': '设备吊销',
}

export default function Settings() {
  const identity = useStore((s) => s.identity)
  const state = useStore((s) => s.state)
  const detail = useStore((s) => s.detail)
  const bots = useStore((s) => s.bots)
  const devices = useStore((s) => s.devices)
  const audit = useStore((s) => s.audit)
  const loadDevices = useStore((s) => s.loadDevices)
  const loadAudit = useStore((s) => s.loadAudit)
  const revokeDevice = useStore((s) => s.revokeDevice)
  const reconnect = useStore((s) => s.reconnect)
  const unpair = useStore((s) => s.unpair)

  const online = state === 'online'

  useEffect(() => {
    if (!online) return
    void loadDevices()
    void loadAudit()
  }, [online, loadDevices, loadAudit])

  if (!identity) return null

  return (
    <Page title="设置">
      <Group title="服务">
        <Row
          title={identity.serviceName || 'LianT Service'}
          subtitle={`${identity.host}:${identity.port}`}
          trailing={<Badge tone={online ? 'ok' : 'warn'}>{online ? '在线' : state === 'reconnecting' ? '重连中' : '离线'}</Badge>}
        />
        <Divider />
        {!online && <Row title="重新连接" subtitle="断开时点这里立刻重试" onClick={() => void reconnect()} />}
        {!online && <Divider />}
        <Row title="服务指纹" subtitle={identity.fp || '未知'} />
        <Divider />
        <Row title="本机设备标识" subtitle={identity.deviceId} />
        {detail && (
          <>
            <Divider />
            <Row title="最近状态" subtitle={detail} />
          </>
        )}
      </Group>

      <Group title="已配对设备" footer="吊销后该设备立刻失去连接权限，需要在它上面重新配对。">
        {devices.length === 0 ? (
          <Row title="暂无设备" subtitle={online ? '列表为空' : '连接后才能读取'} />
        ) : (
          devices.map((d, i) => (
            <div key={d.device_id}>
              {i > 0 && <Divider />}
              <Row
                title={
                  <span className="flex items-center gap-2">
                    {d.name || '未命名设备'}
                    {d.device_id === identity.deviceId && <Badge tone="ok">本机</Badge>}
                  </span>
                }
                subtitle={`最近活动 ${fmtTime(d.last_seen_at)} · ${d.device_id}`}
                trailing={
                  d.device_id === identity.deviceId ? undefined : (
                    <button
                      onClick={() => {
                        if (confirm(`吊销「${d.name || d.device_id}」？`)) void revokeDevice(d.device_id)
                      }}
                      className="h-8 rounded-full bg-[color-mix(in_oklab,var(--danger)_14%,transparent)] px-3 text-[13px] font-medium text-[var(--danger)] active:opacity-60"
                    >
                      吊销
                    </button>
                  )
                }
              />
            </div>
          ))
        )}
      </Group>

      <Group title="安全日志" footer="只保留最近 2000 条，成功与失败都会记录来源 IP。">
        {audit.length === 0 ? (
          <Row title="暂无记录" />
        ) : (
          audit.slice(0, 8).map((e, i) => (
            <div key={`${e.ts}-${i}`}>
              {i > 0 && <Divider />}
              <Row
                title={kindText[e.kind] || e.kind}
                subtitle={[e.name || e.device_id, e.reason, e.ip].filter(Boolean).join(' · ')}
                trailing={<span className="text-[12px] text-[var(--text-3)]">{fmtTime(e.ts)}</span>}
              />
            </div>
          ))
        )}
      </Group>

      <Group title="机器人" footer="多机器人同时在线，事件按 bot_id 区分；通知与提醒由你自己决定看哪一个。">
        <Row title="已配置" subtitle={`${bots.length} 个，运行中 ${bots.filter((b) => b.status === 'running').length} 个`} />
      </Group>

      <Group title="安全">
        <Row title="设备密钥" subtitle="P-256 私钥由浏览器生成，不可导出，仅存本机" />
        <Divider />
        <Row
          title="解除配对"
          danger
          onClick={() => {
            if (confirm('解除后需要重新配对才能连接，确定继续？')) void unpair()
          }}
        />
      </Group>

      <Group title="关于" footer="协议 v1 · 端到端签名握手 · 消息本地留存">
        <Row title="LianT 网页端" subtitle={import.meta.env.DEV ? '开发模式' : '生产构建'} />
      </Group>
    </Page>
  )
}
