#!/bin/sh
# Instala (ou atualiza) o finan_rich no macOS ou Linux, como um aplicativo:
# ícone no menu de aplicativos (Linux) ou em Aplicativos (macOS).
#
#   curl -fsSL https://raw.githubusercontent.com/alexandreserra1/finan_rich/main/scripts/install.sh | sh
#
# Desinstalar (os seus dados ficam):
#   curl -fsSL https://raw.githubusercontent.com/alexandreserra1/finan_rich/main/scripts/install.sh | sh -s -- --uninstall
set -eu

REPO="alexandreserra1/finan_rich"
DEST="${FINAN_INSTALL_DIR:-$HOME/.local/bin}"
APPS="$HOME/.local/share/applications"
ICONS="$HOME/.local/share/icons/hicolor/512x512/apps"
MACAPP="$HOME/Applications/finan_rich.app"

case "$(uname -s)" in
  Darwin) os=macos ;;
  Linux) os=linux ;;
  *) echo "Sistema não suportado por este script. No Windows, use o comando do PowerShell do README." >&2; exit 1 ;;
esac

# Fecha o finan_rich aberto, para a versão nova valer na próxima abertura.
quit_running() {
  curl -fs -X POST -o /dev/null http://127.0.0.1:8080/configuracoes/fechar 2>/dev/null && sleep 1 || true
}

if [ "${1:-}" = "--uninstall" ]; then
  quit_running
  rm -f "$DEST/finanrich" "$APPS/finanrich.desktop" "$ICONS/finanrich.png"
  rm -rf "$MACAPP"
  echo "finan_rich removido. Seus dados continuam na pasta de dados (veja o README) e voltam se você instalar de novo."
  exit 0
fi

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "Processador não suportado: $(uname -m)" >&2; exit 1 ;;
esac

# A última versão sai do redirecionamento de /releases/latest (a API do
# GitHub limita as consultas por IP e falha em redes compartilhadas).
tag=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" | sed -n 's|.*/releases/tag/||p')
if [ -z "$tag" ]; then
  echo "Não encontrei nenhuma versão publicada em https://github.com/$REPO/releases" >&2
  exit 1
fi
version=${tag#v}
file="finan_rich_${version}_${os}_${arch}.tar.gz"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "Baixando finan_rich $tag ($os/$arch)..."
curl -fsSL "https://github.com/$REPO/releases/download/$tag/$file" -o "$tmp/$file"
curl -fsSL "https://github.com/$REPO/releases/download/$tag/checksums.txt" -o "$tmp/checksums.txt"
sum=$(grep " $file\$" "$tmp/checksums.txt" | cut -d' ' -f1)
got=$( (command -v sha256sum >/dev/null && sha256sum "$tmp/$file" || shasum -a 256 "$tmp/$file") | cut -d' ' -f1)
if [ -z "$sum" ] || [ "$sum" != "$got" ]; then
  echo "O arquivo baixado não confere com checksums.txt; nada foi instalado." >&2
  exit 1
fi
tar -xzf "$tmp/$file" -C "$tmp"

quit_running
mkdir -p "$DEST"
install -m 755 "$tmp/finanrich" "$DEST/finanrich"

# Ícone (versões antigas não traziam o ícone no pacote: usa o do repositório).
icon="$tmp/icon.png"
[ -f "$icon" ] || curl -fsSL "https://raw.githubusercontent.com/$REPO/main/packaging/icon.png" -o "$icon" 2>/dev/null || true

if [ "$os" = linux ]; then
  mkdir -p "$APPS" "$ICONS"
  [ -f "$icon" ] && install -m 644 "$icon" "$ICONS/finanrich.png"
  cat > "$APPS/finanrich.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=finan_rich
Comment=Finanças pessoais, com seus dados no seu computador
Exec=$DEST/finanrich
Icon=finanrich
Terminal=false
Categories=Office;Finance;
EOF
  command -v update-desktop-database >/dev/null && update-desktop-database "$APPS" >/dev/null 2>&1 || true
  where="no menu de aplicativos (procure por finan_rich)"
else
  # Um .app mínimo que abre o executável; o ícone vira .icns com as
  # ferramentas do próprio macOS.
  rm -rf "$MACAPP"
  mkdir -p "$MACAPP/Contents/MacOS" "$MACAPP/Contents/Resources"
  printf '#!/bin/sh\nexec "%s/finanrich"\n' "$DEST" > "$MACAPP/Contents/MacOS/finan_rich"
  chmod 755 "$MACAPP/Contents/MacOS/finan_rich"
  if [ -f "$icon" ]; then
    set_dir="$tmp/icon.iconset"
    mkdir -p "$set_dir"
    for s in 16 32 128 256 512; do
      sips -z $s $s "$icon" --out "$set_dir/icon_${s}x${s}.png" >/dev/null 2>&1 || true
    done
    iconutil -c icns "$set_dir" -o "$MACAPP/Contents/Resources/icon.icns" 2>/dev/null || true
  fi
  cat > "$MACAPP/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleName</key><string>finan_rich</string>
  <key>CFBundleIdentifier</key><string>io.github.alexandreserra1.finanrich</string>
  <key>CFBundleExecutable</key><string>finan_rich</string>
  <key>CFBundleIconFile</key><string>icon</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>$version</string>
  <key>LSUIElement</key><true/>
</dict></plist>
EOF
  where="em Aplicativos ($MACAPP) e no Launchpad"
fi

echo
echo "finan_rich $tag instalado. Abra pelo ícone $where."
echo "Pelo terminal: $DEST/finanrich"
echo "Para atualizar, rode o mesmo comando de novo. Seus dados ficam onde estão."
if ! curl -fs -o /dev/null http://localhost:11434/api/tags 2>/dev/null; then
  echo
  echo "Para o consultor com IA grátis, instale também o Ollama: https://ollama.com/download"
fi
