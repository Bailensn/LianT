import DownloadSystem from "@/components/DownloadSystem";
import { serviceBuilds } from "@/data/downloads";

export default function ServicePage() {
  return (
    <DownloadSystem
      title="下载 Service"
      description="服务端组件"
      builds={serviceBuilds}
      product="Service"
      num="02"
    />
  );
}
