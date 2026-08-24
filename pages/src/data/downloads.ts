export interface Asset {
  ext: string
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

const OS_KEY: Record<string, string> = {
  macOS: "Darwin",
}

export function versionFromTag(tag: string): string {
  const part = tag.slice(tag.lastIndexOf("-") + 1).trim()
  return part.startsWith("v") ? part : `v${part}`
}

export function buildReleaseFileName(params: {
  product: string
  tag: string
  os: string
  arch: string
  ext: string
}): string {
  const { product, tag, os, arch, ext } = params
  const osKey = OS_KEY[os] ?? os
  const base = `LianT-${product}-${versionFromTag(tag)}-${osKey}-${arch}`
  return ext ? `${base}.${ext}` : base
}