import DownloadSystem from "@/components/DownloadSystem";
import { managerBuilds } from "@/data/downloads";

export default function ManagerPage() {
  return (
    <DownloadSystem
      title="下载 Manager"
      description="Telegram Bot 管理器客户端"
      builds={managerBuilds}
      product="Manager"
      num="01"
    />
  );
}
