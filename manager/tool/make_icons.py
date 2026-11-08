#!/usr/bin/env python3
"""生成 Android 启动图标、应用内 logo 与网页 favicon（只用标准库）。

设计：大大的圆 + 平滑的蓝紫渐变，中间一个圆头圆角的白色 L。
运行：python3 tool/make_icons.py
"""
import math
import os
import struct
import zlib

SS = 4  # 超采样倍数

# 圆面从左上到右下的径向渐变
GRADIENT_CENTER = (0x5B, 0x8C, 0xFF)
GRADIENT_EDGE = (0x1E, 0x4F, 0xD6)
WHITE = (255, 255, 255)


def _dist_to_segment(px, py, x0, y0, x1, y1):
    dx = x1 - x0
    dy = y1 - y0
    length2 = dx * dx + dy * dy
    if length2 == 0:
        return math.hypot(px - x0, py - y0)
    t = ((px - x0) * dx + (py - y0) * dy) / length2
    t = max(0.0, min(1.0, t))
    return math.hypot(px - (x0 + t * dx), py - (y0 + t * dy))


def _mix(a, b, t):
    t = max(0.0, min(1.0, t))
    return (
        round(a[0] + (b[0] - a[0]) * t),
        round(a[1] + (b[1] - a[1]) * t),
        round(a[2] + (b[2] - a[2]) * t),
    )


def _smoothstep(t):
    t = max(0.0, min(1.0, t))
    return t * t * (3 - 2 * t)


def _render(size, *, with_disc, with_l):
    s = size * SS
    pixels = bytearray(s * s * 4)
    cx = cy = s / 2
    radius = s * 0.47

    if with_disc:
        for y in range(s):
            for x in range(s):
                dx = x + 0.5 - cx
                dy = y + 0.5 - cy
                distance = math.hypot(dx, dy)
                if distance > radius:
                    continue
                color = _mix(GRADIENT_CENTER, GRADIENT_EDGE, _smoothstep(distance / radius))
                # 左上角一层柔和高光，让圆面有平滑过渡的体积感
                glow = max(0.0, 1.0 - math.hypot(dx + radius * 0.35, dy + radius * 0.35) / (radius * 0.95))
                if glow > 0:
                    color = _mix(color, WHITE, glow * glow * 0.22)
                i = (y * s + x) * 4
                pixels[i:i + 4] = bytes((color[0], color[1], color[2], 255))

    if with_l:
        # 旧版 icon.svg 的包围盒（170..342 / 132..380），笔画换成 54 宽圆头折线。
        stroke = 54 / 512 * s
        half = stroke / 2
        x0 = 197 / 512 * s
        y0 = 159 / 512 * s
        y1 = 353 / 512 * s
        x1 = 315 / 512 * s
        for y in range(s):
            py = y + 0.5
            if py < y0 - half or py > y1 + half:
                continue
            for x in range(s):
                px = x + 0.5
                distance = min(
                    _dist_to_segment(px, py, x0, y0, x0, y1),
                    _dist_to_segment(px, py, x0, y1, x1, y1),
                )
                if distance <= half:
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


def _icon(size, *, with_disc, with_l):
    return _downsample(_render(size, with_disc=with_disc, with_l=with_l), size)


def main():
    root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    res = os.path.join(root, 'android', 'app', 'src', 'main', 'res')

    for name, size in {
        'mdpi': 48,
        'hdpi': 72,
        'xhdpi': 96,
        'xxhdpi': 144,
        'xxxhdpi': 192,
    }.items():
        path = os.path.join(res, 'mipmap-' + name, 'ic_launcher.png')
        _write_png(path, size, _icon(size, with_disc=True, with_l=True))
        print('wrote', path)

    # Android 8+ 自适应图标：渐变底 + 白色前景（L 放在安全区内）。
    for name, size in {
        'mdpi': 108,
        'hdpi': 162,
        'xhdpi': 216,
        'xxhdpi': 324,
        'xxxhdpi': 432,
    }.items():
        path = os.path.join(res, 'mipmap-' + name, 'ic_launcher_foreground.png')
        _write_png(path, size, _icon(size, with_disc=False, with_l=True))
        print('wrote', path)

    adaptive_dir = os.path.join(res, 'mipmap-anydpi-v26')
    os.makedirs(adaptive_dir, exist_ok=True)
    with open(os.path.join(adaptive_dir, 'ic_launcher.xml'), 'w') as handle:
        handle.write('''<?xml version="1.0" encoding="utf-8"?>
<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android">
    <background android:drawable="@drawable/ic_launcher_background"/>
    <foreground android:drawable="@mipmap/ic_launcher_foreground"/>
</adaptive-icon>
''')

    drawable_dir = os.path.join(res, 'drawable')
    os.makedirs(drawable_dir, exist_ok=True)
    with open(os.path.join(drawable_dir, 'ic_launcher_background.xml'), 'w') as handle:
        handle.write('''<?xml version="1.0" encoding="utf-8"?>
<shape xmlns:android="http://schemas.android.com/apk/res/android"
    android:shape="rectangle">
    <gradient
        android:type="radial"
        android:gradientRadius="75%p"
        android:centerX="0.35"
        android:centerY="0.3"
        android:startColor="#5B8CFF"
        android:endColor="#1E4FD6" />
</shape>
''')

    master = os.path.join(root, 'assets', 'icon.png')
    _write_png(master, 512, _icon(512, with_disc=True, with_l=True))
    print('wrote', master)

    favicon = os.path.join(root, 'web', 'favicon.png')
    _write_png(favicon, 64, _icon(64, with_disc=True, with_l=True))
    print('wrote', favicon)


if __name__ == '__main__':
    main()
