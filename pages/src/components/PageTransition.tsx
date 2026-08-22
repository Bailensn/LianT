"use client";

import { usePathname } from "next/navigation";

/** 页面切换动画：路径变化时重新挂载子内容，触发淡入动画 */
export default function PageTransition({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  return (
    <div key={pathname} className="page-transition">
      {children}
    </div>
  );
}
