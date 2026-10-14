"""Draws the Modbus Studio icon: a bus line with three devices and a data waveform.

Usage: python build/make_icon.py <out_dir>
Writes appicon.png (1024), icon.ico (16-256, simplified art below 48 px) and preview.png.
Copy appicon.png to build/ and icon.ico to build/windows/. Wails makes the macOS icon from appicon.png.
"""
import io
import struct
import sys
from PIL import Image, ImageDraw

SS = 4  # supersampling factor

TOP = (43, 125, 233)     # gradient top
BOTTOM = (8, 72, 160)    # gradient bottom
WHITE = (255, 255, 255, 255)
AMBER = (255, 197, 66, 255)
WAVE = (168, 209, 255, 255)


def render(size, simple):
    S = 1024 * SS
    k = SS
    img = Image.new('RGBA', (S, S), (0, 0, 0, 0))

    # Rounded-square background with a vertical gradient.
    grad = Image.new('RGBA', (S, S))
    gd = ImageDraw.Draw(grad)
    for y in range(S):
        t = y / (S - 1)
        gd.line([(0, y), (S, y)], fill=tuple(int(TOP[i] + (BOTTOM[i] - TOP[i]) * t) for i in range(3)) + (255,))
    mask = Image.new('L', (S, S), 0)
    m = 64 if simple else 96
    ImageDraw.Draw(mask).rounded_rectangle([m * k, m * k, (1024 - m) * k, (1024 - m) * k], radius=(170 if simple else 200) * k, fill=255)
    img.paste(grad, (0, 0), mask)

    d = ImageDraw.Draw(img)
    R = lambda *v: [x * k for x in v]
    if simple:
        bus_y, bus_h, tap_w, dev, dev_r, xs = 330, 84, 64, 196, 40, (290, 512, 734)
        bus_x = (170, 854)
    else:
        bus_y, bus_h, tap_w, dev, dev_r, xs = 450, 56, 44, 136, 30, (322, 512, 702)
        bus_x = (210, 814)
        # Data waveform above the bus.
        lo, hi = 370, 290
        pts = [(250, lo), (330, lo), (330, hi), (430, hi), (430, lo), (490, lo), (490, hi), (590, hi),
               (590, lo), (650, lo), (650, hi), (700, hi), (700, lo), (774, lo)]
        d.line([(x * k, y * k) for x, y in pts], fill=WAVE, width=34 * k, joint='curve')
        for x, y in (pts[0], pts[-1]):
            d.ellipse(R(x - 17, y - 17, x + 17, y + 17), fill=WAVE)

    # Bus line.
    d.rounded_rectangle(R(bus_x[0], bus_y, bus_x[1], bus_y + bus_h), radius=bus_h // 2 * k, fill=WHITE)
    tap_top = bus_y + bus_h
    dev_top = tap_top + (110 if simple else 94)
    for i, x in enumerate(xs):
        d.rectangle(R(x - tap_w // 2, tap_top - 4, x + tap_w // 2, dev_top + 4), fill=WHITE)
        d.rounded_rectangle(R(x - dev // 2, dev_top, x + dev // 2, dev_top + dev), radius=dev_r * k, fill=AMBER if i == 1 else WHITE)

    return img.resize((size, size), Image.LANCZOS)


def write_ico(path, images):
    """ICO with one PNG-compressed entry per size, so small sizes keep their own art."""
    blobs = []
    for im in images:
        b = io.BytesIO()
        im.save(b, 'PNG')
        blobs.append((im.size[0], b.getvalue()))
    out = io.BytesIO()
    out.write(struct.pack('<HHH', 0, 1, len(blobs)))
    offset = 6 + 16 * len(blobs)
    for s, data in blobs:
        out.write(struct.pack('<BBBBHHII', 0 if s >= 256 else s, 0 if s >= 256 else s, 0, 0, 1, 32, len(data), offset))
        offset += len(data)
    for _, data in blobs:
        out.write(data)
    open(path, 'wb').write(out.getvalue())


if __name__ == '__main__':
    out = sys.argv[1]
    render(1024, False).save(f'{out}/appicon.png')
    sizes = [16, 24, 32, 48, 64, 128, 256]
    write_ico(f'{out}/icon.ico', [render(s, s < 48) for s in sizes])
    # Preview sheet: large icon plus each ICO size, on light and dark strips.
    sheet = Image.new('RGBA', (1024 + 40 + 560, 1024), (245, 247, 250, 255))
    sheet.paste(render(1024, False), (0, 0))
    y = 20
    for bg in ((245, 247, 250, 255), (30, 34, 40, 255)):
        ImageDraw.Draw(sheet).rectangle([1064, y - 10, 1624, y + 290], fill=bg)
        x = 1074
        for s in [16, 24, 32, 48, 64, 128]:
            im = render(s, s < 48)
            sheet.paste(im, (x, y + 130 - s // 2), im)
            x += s + 18
        y += 330
    sheet.save(f'{out}/preview.png')
    print('ok')
