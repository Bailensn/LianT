#!/usr/bin/env python3
"""生成 Android 启动图标与网页 favicon（只用标准库）。

设计：品牌蓝 #2F6BFF 圆角方块 + 白色 L，与应用内 logo 一致。
运行：python3 tool/make_icons.py
"""
import os
import struct
import zlib

BLUE = (0x2F, 0x6B, 0xFF)
WHITE = (255, 255, 255)
SS = 4  # 超采样倍数


def _inside_round_rect(px, py, x0, y0, x1, y1, radius):
    cx = min(max(px, x0 + radius), x1 - radius)
    cy = min(max(py, y0 + radius), y1 - radius)
    return (px - cx) ** 2 + (py - cy) ** 2 <= radius * radius


def _render(size, *, background, with_l):
    """返回 size*SS 的 RGBA 像素。"""
    s = size * SS
    pixels = bytearray(s * s * 4)

    if background is not None:
        radius = 112 / 512 * s
        for y in range(s):
            for x in range(s):
                if _inside_round_rect(x + 0.5, y + 0.5, 0, 0, s, s, radius):
                    i = (y * s + x) * 4
                    pixels[i:i + 4] = bytes((background[0], background[1], background[2], 255))

    if with_l:
        # 与旧版 web/public/icon.svg 完全一致的 L：
        # M170 132h44v206h128v42H170z
        stem = (170 / 512 * s, 132 / 512 * s, 214 / 512 * s, 338 / 512 * s)
        foot = (170 / 512 * s, 338 / 512 * s, 342 / 512 * s, 380 / 512 * s)
        for y in range(s):
            py = y + 0.5
            if py < stem[1] or py > foot[3]:
                continue
            for x in range(s):
                px = x + 0.5
                if _inside_round_rect(px, py, *stem, 0) or _inside_round_rect(px, py, *foot, 0):
                    i = (y * s + x) * 4
                    pixels[i:i + 4] = bytes((WHITE[0], WHITE[1], WHITE[2], 255))
    return pixels


def _downsample(pixels, size):
    out = bytearray(size * size * 4)
    hi = size * SS
    for y in range(size):
        for x in range(size):
            sum_r = sum_g = sum_b = sum_a = 0
            for dy in range(SS):
                row = ((y * SS + dy) * hi + x * SS) * 4
                for dx in range(SS):
                    i = row + dx * 4
                    alpha = pixels[i + 3]
                    sum_r += pixels[i] * alpha
                    sum_g += pixels[i + 1] * alpha
                    sum_b += pixels[i + 2] * alpha
                    sum_a += alpha
            o = (y * size + x) * 4
            if sum_a > 0:
                out[o] = sum_r // sum_a
                out[o + 1] = sum_g // sum_a
                out[o + 2] = sum_b // sum_a
            out[o + 3] = sum_a // (SS * SS)
    return out


def _write_png(path, size, pixels):
    def chunk(tag, data):
        body = tag + data
        return struct.pack('>I', len(data)) + body + struct.pack('>I', zlib.crc32(body) & 0xFFFFFFFF)

    raw = b''.join(
        b'\x00' + bytes(pixels[y * size * 4:(y + 1) * size * 4]) for y in range(size)
    )
    png = b'\x89PNG\r\n\x1a\n'
    png += chunk(b'IHDR', struct.pack('>IIBBBBB', size, size, 8, 6, 0, 0, 0))
    png += chunk(b'IDAT', zlib.compress(raw, 9))
    png += chunk(b'IEND', b'')
    os.makedirs(os.path.dirname(path) or '.', exist_ok=True)
    with open(path, 'wb') as handle:
        handle.write(png)


def _icon(size, *, background, with_l):
    return _downsample(_render(size, background=background, with_l=with_l), size)


def main():
    root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    res = os.path.join(root, 'android', 'app', 'src', 'main', 'res')
    densities = {
        'mdpi': 48,
        'hdpi': 72,
        'xhdpi': 96,
        'xxhdpi': 144,
        'xxxhdpi': 192,
    }
    for name, size in densities.items():
        path = os.path.join(res, 'mipmap-' + name, 'ic_launcher.png')
        _write_png(path, size, _icon(size, background=BLUE, with_l=True))
        print('wrote', path)

    # Android 8+ 自适应图标：纯色底 + 白色前景，安全区约 66%。
    foreground = {
        'mdpi': 108,
        'hdpi': 162,
        'xhdpi': 216,
        'xxhdpi': 324,
        'xxxhdpi': 432,
    }
    for name, size in foreground.items():
        path = os.path.join(res, 'mipmap-' + name, 'ic_launcher_foreground.png')
        _write_png(path, size, _icon(size, background=None, with_l=True))
        print('wrote', path)

    adaptive_dir = os.path.join(res, 'mipmap-anydpi-v26')
    os.makedirs(adaptive_dir, exist_ok=True)
    with open(os.path.join(adaptive_dir, 'ic_launcher.xml'), 'w') as handle:
        handle.write('''<?xml version="1.0" encoding="utf-8"?>
<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android">
    <background android:drawable="@color/ic_launcher_background"/>
    <foreground android:drawable="@mipmap/ic_launcher_foreground"/>
</adaptive-icon>
''')

    values_dir = os.path.join(res, 'values')
    os.makedirs(values_dir, exist_ok=True)
    with open(os.path.join(values_dir, 'colors.xml'), 'w') as handle:
        handle.write('''<?xml version="1.0" encoding="utf-8"?>
<resources>
    <color name="ic_launcher_background">#2F6BFF</color>
</resources>
''')

    master = os.path.join(root, 'assets', 'icon.png')
    _write_png(master, 512, _icon(512, background=BLUE, with_l=True))
    print('wrote', master)

    favicon = os.path.join(root, 'web', 'favicon.png')
    _write_png(favicon, 64, _icon(64, background=BLUE, with_l=True))
    print('wrote', favicon)


if __name__ == '__main__':
    main()
