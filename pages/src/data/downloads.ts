export interface Asset {
  /** 文件后缀，例如 "deb"、"apk"。无后缀的可执行文件（如 Service 的 Linux / macOS 二进制）传空字符串 */
  ext: string
  /** 界面上展示的格式标记，例如 ".deb"、".exe" */
  label: string
}

export interface Build {
  os: string
  arch: string
  assets: Asset[]
}

export const managerBuilds: Build[] = [
  {
    os: "Windows",
    arch: "amd64",
    assets: [{ ext: "exe", label: ".exe" }],
  },
  {
    os: "Linux",
    arch: "amd64",
    assets: [
      { ext: "deb", label: ".deb" },
      { ext: "rpm", label: ".rpm" },
    ],
  },
  {
    os: "Linux",
    arch: "arm64",
    assets: [
      { ext: "deb", label: ".deb" },
      { ext: "rpm", label: ".rpm" },
    ],
  },
  {
    os: "macOS",
    arch: "arm64",
    assets: [{ ext: "dmg", label: ".dmg" }],
  },
  {
    os: "Android",
    arch: "arm64",
    assets: [{ ext: "apk", label: ".apk" }],
  },
]

export const serviceBuilds: Build[] = [
  {
    os: "Windows",
    arch: "amd64",
    assets: [{ ext: "exe", label: ".exe" }],
  },
  {
    os: "Linux",
    arch: "amd64",
    assets: [{ ext: "", label: "可执行文件" }],
  },
  {
    os: "Linux",
    arch: "arm64",
    assets: [{ ext: "", label: "可执行文件" }],
  },
  {
    os: "macOS",
    arch: "arm64",
    assets: [{ ext: "", label: "可执行文件" }],
  },
]

/** 展示用系统名 -> 产物文件名中的系统标识（GitHub 侧按 GOOS 命名，macOS 对应 Darwin） */
const OS_KEY: Record<string, string> = {
  macOS: "Darwin",
}

/** 版本号统一补上 v 前缀（release.name 可能是 "0.0.1" 而 tag_name 是 "v0.0.1"） */
function withVersionPrefix(version: string): string {
  return version.startsWith("v") ? version : `v${version}`
}

/**
 * 拼接 Release 产物文件名。
 * - Manager：LianT-Manager-v0.0.1-Linux_arm64.deb
 * - Service：除 Windows 外均无后缀，例如 LianT-Service-v0.0.1-Linux_arm64
 */
export function buildReleaseFileName(params: {
  product: string
  version: string
  os: string
  arch: string
  ext: string
}): string {
  const { product, version, os, arch, ext } = params
  const osKey = OS_KEY[os] ?? os
  // 实际产物中 os 与 arch 之间用连字符分隔：LianT-Manager-v0.01-Linux-amd64.deb
  const base = `LianT-${product}-${withVersionPrefix(version)}-${osKey}-${arch}`
  return ext ? `${base}.${ext}` : base
}