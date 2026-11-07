import jsQR from 'jsqr'

// 从相册图片里识别二维码，限制边长避免大图吃内存。
export async function decodeQr(file: File): Promise<string> {
  const bitmap = await createImageBitmap(file)
  const scale = Math.min(1, 1400 / Math.max(bitmap.width, bitmap.height))
  const w = Math.round(bitmap.width * scale)
  const h = Math.round(bitmap.height * scale)
  const canvas = document.createElement('canvas')
  canvas.width = w
  canvas.height = h
  const ctx = canvas.getContext('2d', { willReadFrequently: true })
  if (!ctx) throw new Error('无法读取这张图片')
  ctx.drawImage(bitmap, 0, 0, w, h)
  bitmap.close()

  const image = ctx.getImageData(0, 0, w, h)
  const code = jsQR(image.data, w, h, { inversionAttempts: 'attemptBoth' })
  if (!code) throw new Error('图片里没有找到二维码')
  return code.data
}
