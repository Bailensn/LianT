import type { Metadata } from "next";
import "./globals.css";
import Nav from "@/components/Nav";
import PageTransition from "@/components/PageTransition";

export const metadata: Metadata = {
  title: "LianT · 联T",
  description: "Telegram Bot 管理器与服务端组件。服务器自备。",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN">
      <body>
        <div className="noise" aria-hidden="true" />
        <Nav />
        <PageTransition>{children}</PageTransition>
      </body>
    </html>
  );
}
