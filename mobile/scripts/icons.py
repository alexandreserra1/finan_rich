# Gera os ícones e a tela de abertura do app Android a partir do desenho do favicon.svg.
# Uso (precisa do Pillow): python3 mobile/scripts/icons.py mobile/android/app/src/main/res [mobile/ios/App/App/Assets.xcassets]
import sys, os
from PIL import Image, ImageDraw
res = sys.argv[1]
GREEN = (15, 122, 92, 255)
PTS = [(8, 22), (13, 16), (17, 19), (24, 10)]

def line(draw, scale, ox, oy, width):
    pts = [(ox + x * scale, oy + y * scale) for x, y in PTS]
    draw.line(pts, fill="white", width=int(width), joint="curve")
    r = width / 2
    for x, y in pts:
        draw.ellipse([x - r, y - r, x + r, y + r], fill="white")

def render(size, shape, ss=4):
    S = size * ss
    img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    if shape == "square":
        d.rounded_rectangle([0, 0, S - 1, S - 1], radius=S * 7 / 32, fill=GREEN)
    elif shape == "round":
        d.ellipse([0, 0, S - 1, S - 1], fill=GREEN)
    if shape == "foreground":
        # 108dp com a área segura de 66dp no centro: o desenho ocupa ~90% dela.
        scale = S * 0.9 / 32 * (66 / 108)
        ox = oy = S / 2 - 16 * scale
        line(d, scale, ox, oy, 3 * scale)
    else:
        scale = S / 32
        line(d, scale, 0, 0, 3 * scale)
    return img.resize((size, size), Image.LANCZOS)

DENS = {"mdpi": 1, "hdpi": 1.5, "xhdpi": 2, "xxhdpi": 3, "xxxhdpi": 4}
for name, f in DENS.items():
    d = os.path.join(res, "mipmap-" + name)
    render(int(48 * f), "square").save(os.path.join(d, "ic_launcher.png"))
    render(int(48 * f), "round").save(os.path.join(d, "ic_launcher_round.png"))
    render(int(108 * f), "foreground").save(os.path.join(d, "ic_launcher_foreground.png"))

# Telas de abertura: fundo verde com o ícone no centro.
for folder in os.listdir(res):
    p = os.path.join(res, folder, "splash.png")
    if not os.path.exists(p):
        continue
    w, h = Image.open(p).size
    img = Image.new("RGB", (w, h), GREEN[:3])
    icon = render(int(min(w, h) * 0.3), "none")
    img.paste(icon, ((w - icon.width) // 2, (h - icon.height) // 2), icon)
    img.save(p)

# iOS: ícone de 1024 px sem cantos arredondados (o sistema arredonda) e telas de abertura.
if len(sys.argv) > 2:
    xc = sys.argv[2]
    icon = render(1024, "none")
    full = Image.new("RGB", (1024, 1024), GREEN[:3])
    full.paste(icon, (0, 0), icon)
    full.save(os.path.join(xc, "AppIcon.appiconset", "AppIcon-512@2x.png"))
    splash_dir = os.path.join(xc, "Splash.imageset")
    for name in os.listdir(splash_dir):
        if name.endswith(".png"):
            p = os.path.join(splash_dir, name)
            w, h = Image.open(p).size
            img = Image.new("RGB", (w, h), GREEN[:3])
            ic = render(int(min(w, h) * 0.3), "none")
            img.paste(ic, ((w - ic.width) // 2, (h - ic.height) // 2), ic)
            img.save(p)
