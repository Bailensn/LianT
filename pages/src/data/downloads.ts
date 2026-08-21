export interface Build {
  os: string
  arch: string
  formats: string[]
}

export const managerBuilds: Build[] = [
  {
    os: "Windows",
    arch: "amd64",
    formats: ["exe"],
  },
  {
    os: "Linux",
    arch: "amd64",
    formats: ["deb", "rpm"],
  },
  {
    os: "Linux",
    arch: "arm64",
    formats: ["deb", "rpm"],
  },
  {
    os: "macOS",
    arch: "arm64",
    formats: ["dmg"],
  },
  {
    os: "Android",
    arch: "arm64",
    formats: ["apk"],
  },
]

export const serviceBuilds: Build[] = [
  {
    os: "Windows",
    arch: "amd64",
    formats: ["exe"],
  },
  {
    os: "Linux",
    arch: "amd64",
    formats: ["deb", "rpm"],
  },
  {
    os: "Linux",
    arch: "arm64",
    formats: ["deb", "rpm"],
  },
  {
    os: "macOS",
    arch: "arm64",
    formats: ["dmg"],
  },
]