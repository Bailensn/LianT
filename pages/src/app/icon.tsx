import { LOGO_PATHS } from "@/lib/logo";

export const contentType = "image/svg+xml";
export const size = { width: 32, height: 32 };

/** 站点 favicon（由品牌 Logo 生成） */
export default function Icon() {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 2000 2000" width="32" height="32">${LOGO_PATHS.map(
    (p) => `<path fill="${p.fill}" d="${p.d}"/>`
  ).join("")}</svg>`;

  return new Response(svg, {
    headers: { "Content-Type": "image/svg+xml" },
  });
}
