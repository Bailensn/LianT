// 仓库地址（来自 service/go.mod 的模块路径 github.com/Bailensn/LianT）
export const REPO_URL = 'https://github.com/Bailensn/LianT'
export const RELEASES_URL = `${REPO_URL}/releases/latest`

// -----------------------------------------------------------------------
// 下载链接说明：
// 在正式发布产物之前，下面所有卡片的 url 都指向 GitHub Releases 页面，
// 用户可以在那里看到当前所有已发布的文件。
//
// 等你在 GitHub Releases 中发布了具体的安装包后，可以把某一项的 url
// 换成指向具体文件的直链，GitHub 支持“latest + 文件名”的固定直链格式：
//   `${RELEASES_URL}/download/<发布时的文件名>`
// 例如：`${RELEASES_URL}/download/liant-service-windows-amd64.exe`
// 这样每次发新版本，只要文件名不变，链接就始终指向最新版本。
// -----------------------------------------------------------------------

export const serviceBuilds = [
  { os: 'Windows', arch: 'amd64', format: 'exe', note: '独立可执行文件', url: RELEASES_URL },
  { os: 'Linux', arch: 'amd64', format: 'deb', note: 'Debian / Ubuntu 系', url: RELEASES_URL },
  { os: 'Linux', arch: 'amd64', format: 'rpm', note: 'Fedora / RHEL 系', url: RELEASES_URL },
  { os: 'Linux', arch: 'arm64', format: 'deb', note: 'Debian / Ubuntu 系', url: RELEASES_URL },
  { os: 'Linux', arch: 'arm64', format: 'rpm', note: 'Fedora / RHEL 系', url: RELEASES_URL },
  { os: 'macOS', arch: 'arm64', format: 'dmg', note: 'Apple Silicon', url: RELEASES_URL },
]

export const managerBuilds = [
  { os: 'Windows', arch: 'amd64', format: 'exe', note: '独立可执行文件', url: RELEASES_URL },
  { os: 'Linux', arch: 'amd64', format: 'deb', note: 'Debian / Ubuntu 系', url: RELEASES_URL },
  { os: 'Linux', arch: 'amd64', format: 'rpm', note: 'Fedora / RHEL 系', url: RELEASES_URL },
  { os: 'Linux', arch: 'arm64', format: 'deb', note: 'Debian / Ubuntu 系', url: RELEASES_URL },
  { os: 'Linux', arch: 'arm64', format: 'rpm', note: 'Fedora / RHEL 系', url: RELEASES_URL },
  { os: 'macOS', arch: 'arm64', format: 'dmg', note: 'Apple Silicon', url: RELEASES_URL },
  { os: 'Android', arch: 'arm64', format: 'apk', note: '手机 / 平板', url: RELEASES_URL },
]
