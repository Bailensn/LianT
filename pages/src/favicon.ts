import { buildLogoSvg } from "./logo"

export function setupFavicon() {
  const svg = buildLogoSvg(64)
  const dataUri = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`

  const link = document.createElement("link")
  link.rel = "icon"
  link.type = "image/svg+xml"
  link.href = dataUri
  document.head.appendChild(link)
}