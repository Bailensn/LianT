import { useStore } from '../store'
import { Page, Group, Row, Divider, Badge } from '../ui/Common'

export default function Settings() {
  const identity = useStore((s) => s.identity)
  const state = useStore((s) => s.state)
  const detail = useStore((s) => s.detail)
  const bots = useStore((s) => s.bots)
  const unpair = useStore((s) => s.unpair)
  const reconnect = useStore((s) => s.reconnect)

  if (!identity) return null
  const online = state === 'online'

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
        <Row title="设备标识" subtitle={identity.deviceId} />
        {detail && (
          <>
            <Divider />
            <Row title="最近状态" subtitle={detail} />
          </>
        )}
      </Group>

      <Group title="机器人" footer="多机器人同时在线，事件按 bot_id 区分；通知与提醒由你自己决定看哪一个。">
        <Row title="已配置" subtitle={`${bots.length} 个，运行中 ${bots.filter((b) => b.status === 'running').length} 个`} />
      </Group>

      <Group title="安全">
        <Row
          title="设备密钥"
          subtitle="P-256 私钥由浏览器生成，不可导出，仅存本机"
        />
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
