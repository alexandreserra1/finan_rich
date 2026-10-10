#!/bin/sh
# Instala o finan_rich no macOS ou Linux:
#   curl -fsSL https://raw.githubusercontent.com/alexandreserra1/finan_rich/main/scripts/install.sh | sh
set -eu

REPO="alexandreserra1/finan_rich"
DEST="${FINAN_INSTALL_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
  Darwin) os=macos ;;
  Linux) os=linux ;;
  *) echo "Sistema não suportado por este script. No Windows, baixe o .zip em https://github.com/$REPO/releases" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "Processador não suportado: $(uname -m)" >&2; exit 1 ;;
esac

tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
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
tar -xzf "$tmp/$file" -C "$tmp"
mkdir -p "$DEST"
install -m 755 "$tmp/finanrich" "$DEST/finanrich"

echo "Instalado em $DEST/finanrich"
case ":$PATH:" in
  *":$DEST:"*) echo "Para abrir, rode: finanrich" ;;
  *) echo "Para abrir, rode: $DEST/finanrich   (ou adicione $DEST ao seu PATH)" ;;
esac
if ! curl -fs -o /dev/null http://localhost:11434/api/tags 2>/dev/null; then
  echo
  echo "Para o consultor com IA grátis, instale também o Ollama: https://ollama.com/download"
fi
