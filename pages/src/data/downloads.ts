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

/**
 * 从 tag 中提取版本号，并补上 v 前缀。
 * tag 形如 "Manager-v0.01" / "Service-v0.01"，取最后一个 "-" 之后的部分，
 * 即 "v0.01"（若写成 "0.01" 也会补成 "v0.01"）。
 * 优先使用 tag 而非 Release 标题，因为标题可能是随意的非版本描述。
 */
export function versionFromTag(tag: string): string {
  const part = tag.slice(tag.lastIndexOf("-") + 1).trim()
  return part.startsWith("v") ? part : `v${part}`
}

/**
 * 拼接 Release 产物文件名。
 * - Manager：LianT-Manager-v0.01-Linux-amd64.deb
 * - Service：除 Windows 外均无后缀，例如 LianT-Service-v0.01-Linux-amd64
 */
export function buildReleaseFileName(params: {
  product: string
  tag: string
  os: string
  arch: string
  ext: string
}): string {
  const { product, tag, os, arch, ext } = params
  const osKey = OS_KEY[os] ?? os
  // 版本号取自 tag（优先级高于 Release 标题）；os 与 arch 之间用连字符分隔
  const base = `LianT-${product}-${versionFromTag(tag)}-${osKey}-${arch}`
  return ext ? `${base}.${ext}` : base
}