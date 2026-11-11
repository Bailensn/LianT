#!/usr/bin/env python3
"""像素对比：Flutter 截图 vs Compose 截图。

只依赖标准库（zlib/struct），不需要 PIL。

用法：
    python3 manager/tool/ui_diff.py flutter.png native.png [diff.png]

输出：
    - 尺寸、平均通道差、差异像素占比；
    - 差异像素的包围盒（定位是哪一块位置不对）；
    - 可选的 diff.png 热力图：相同处白、差异处红色。

判定阈值默认 8（容忍文字抗锯齿与 0.5px 线的亚像素差），可用 --threshold 改。
"""

from __future__ import annotations

import argparse
import struct
import sys
import zlib


def read_png(path: str) -> tuple[int, int, list[bytearray]]:
    with open(path, "rb") as handle:
        data = handle.read()
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise SystemExit(f"{path} 不是 PNG")

    pos = 8
    width = height = bit_depth = color_type = interlace = 0
    idat = bytearray()
    while pos < len(data):
        length = struct.unpack(">I", data[pos : pos + 4])[0]
        kind = data[pos + 4 : pos + 8]
        body = data[pos + 8 : pos + 8 + length]
        pos += 12 + length
        if kind == b"IHDR":
            width, height, bit_depth, color_type, _, _, interlace = struct.unpack(
                ">IIBBBBB", body
            )
        elif kind == b"IDAT":
            idat += body
        elif kind == b"IEND":
            break

    if bit_depth != 8 or interlace != 0:
        raise SystemExit("只支持 8 位、非隔行 PNG（screencap 默认就是）")
    channels = {0: 1, 2: 3, 4: 2, 6: 4}.get(color_type)
    if channels is None:
        raise SystemExit(f"不支持的颜色类型 {color_type}")

    raw = zlib.decompress(bytes(idat))
    stride = width * channels
    rows: list[bytearray] = []
    prev = bytearray(stride)
    offset = 0
    for _ in range(height):
        filter_type = raw[offset]
        offset += 1
        line = bytearray(raw[offset : offset + stride])
        offset += stride
        if filter_type == 1:  # Sub
            for i in range(channels, stride):
                line[i] = (line[i] + line[i - channels]) & 0xFF
        elif filter_type == 2:  # Up
            for i in range(stride):
                line[i] = (line[i] + prev[i]) & 0xFF
        elif filter_type == 3:  # Average
            for i in range(stride):
                left = line[i - channels] if i >= channels else 0
                line[i] = (line[i] + ((left + prev[i]) >> 1)) & 0xFF
        elif filter_type == 4:  # Paeth
            for i in range(stride):
                left = line[i - channels] if i >= channels else 0
                up = prev[i]
                up_left = prev[i - channels] if i >= channels else 0
                p = left + up - up_left
                pa, pb, pc = abs(p - left), abs(p - up), abs(p - up_left)
                predictor = left if (pa <= pb and pa <= pc) else (up if pb <= pc else up_left)
                line[i] = (line[i] + predictor) & 0xFF
        rows.append(line)
        prev = line

    rgb: list[bytearray] = []
    for line in rows:
        out = bytearray(width * 3)
        for x in range(width):
            base = x * channels
            if channels >= 3:
                out[x * 3 : x * 3 + 3] = line[base : base + 3]
            else:
                gray = line[base]
                out[x * 3 : x * 3 + 3] = bytes((gray, gray, gray))
        rgb.append(out)
    return width, height, rgb


def write_png(path: str, width: int, height: int, rows: list[bytearray]) -> None:
    def chunk(kind: bytes, body: bytes) -> bytes:
        return (
            struct.pack(">I", len(body))
            + kind
            + body
            + struct.pack(">I", zlib.crc32(kind + body) & 0xFFFFFFFF)
        )

    raw = bytearray()
    for row in rows:
        raw.append(0)
        raw += row
    data = b"\x89PNG\r\n\x1a\n"
    data += chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
    data += chunk(b"IDAT", zlib.compress(bytes(raw), 6))
    data += chunk(b"IEND", b"")
    with open(path, "wb") as handle:
        handle.write(data)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("a")
    parser.add_argument("b")
    parser.add_argument("diff", nargs="?", default=None)
    parser.add_argument("--threshold", type=int, default=8)
    args = parser.parse_args()

    wa, ha, a = read_png(args.a)
    wb, hb, b = read_png(args.b)
    if (wa, ha) != (wb, hb):
        print(f"尺寸不同：{wa}x{ha} vs {wb}x{hb}（按左上角对齐比较重叠部分）")
    width, height = min(wa, wb), min(ha, hb)

    diff_pixels = 0
    total_abs = 0
    min_x, min_y = width, height
    max_x = max_y = -1
    heat: list[bytearray] = [bytearray(width * 3) for _ in range(height)]

    for y in range(height):
        ra, rb = a[y], b[y]
        rh = heat[y]
        for x in range(width):
            i = x * 3
            d0 = abs(ra[i] - rb[i])
            d1 = abs(ra[i + 1] - rb[i + 1])
            d2 = abs(ra[i + 2] - rb[i + 2])
            total_abs += d0 + d1 + d2
            worst = max(d0, d1, d2)
            if worst > args.threshold:
                diff_pixels += 1
                if x < min_x:
                    min_x = x
                if y < min_y:
                    min_y = y
                if x > max_x:
                    max_x = x
                if y > max_y:
                    max_y = y
                intensity = min(255, 80 + worst * 2)
                rh[i] = 255
                rh[i + 1] = 255 - intensity
                rh[i + 2] = 255 - intensity
            else:
                rh[i] = rh[i + 1] = rh[i + 2] = 255

    total = width * height
    ratio = diff_pixels / total if total else 0.0
    mean = total_abs / (total * 3) if total else 0.0
    print(f"比较区域：{width}x{height}")
    print(f"差异像素：{diff_pixels}/{total}（{ratio * 100:.4f}%）")
    print(f"平均通道差：{mean:.3f}")
    if max_x >= 0:
        print(f"差异包围盒：x {min_x}..{max_x}，y {min_y}..{max_y}")
    else:
        print("差异包围盒：无（阈值内完全一致）")

    if args.diff:
        write_png(args.diff, width, height, heat)
        print(f"热力图：{args.diff}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
