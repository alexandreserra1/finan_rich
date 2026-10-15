#!/bin/sh
# Cria a chave de assinatura do app Android (uma vez só) e, se o GitHub CLI
# (gh) estiver instalado e logado, já cadastra os 4 secrets que o workflow de
# release usa. Só precisa do OpenSSL (vem no macOS e no Linux; no Windows, use
# o Git Bash).
#
#   sh scripts/android-key.sh
#
# A chave e a senha ficam na pasta finan_rich-chave/ (fora do git). GUARDE
# essa pasta: sem ela, nenhuma versão nova do app atualiza por cima da antiga.
set -eu
cd "$(dirname "$0")/.."

DIR=finan_rich-chave
KEY="$DIR/finan_rich.p12"
ALIAS=finan_rich

if [ -e "$KEY" ]; then
  echo "Já existe $KEY. Não criei outra: trocar a chave impede atualizar o app." >&2
  exit 1
fi
command -v openssl >/dev/null || { echo "Instale o OpenSSL (no Windows, rode pelo Git Bash)." >&2; exit 1; }

mkdir -p "$DIR"
chmod 700 "$DIR"
pass=$(openssl rand -hex 24)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# Chave RSA de 4096 bits num certificado de ~27 anos, no formato PKCS#12
# (que o Android/Gradle aceita como keystore).
openssl req -x509 -newkey rsa:4096 -nodes -days 10000 -subj "/CN=finan_rich" \
  -keyout "$tmp/key.pem" -out "$tmp/cert.pem" 2>/dev/null
openssl pkcs12 -export -name "$ALIAS" -inkey "$tmp/key.pem" -in "$tmp/cert.pem" \
  -out "$KEY" -passout "pass:$pass"
chmod 600 "$KEY"
printf 'ANDROID_KEYSTORE_PASSWORD=%s\nANDROID_KEY_ALIAS=%s\nANDROID_KEY_PASSWORD=%s\n' "$pass" "$ALIAS" "$pass" > "$DIR/senhas.txt"
chmod 600 "$DIR/senhas.txt"
echo "Chave criada em $KEY (senhas em $DIR/senhas.txt)."

if command -v gh >/dev/null && gh auth status >/dev/null 2>&1; then
  openssl base64 -A -in "$KEY" | gh secret set ANDROID_KEYSTORE_BASE64
  printf %s "$pass" | gh secret set ANDROID_KEYSTORE_PASSWORD
  printf %s "$ALIAS" | gh secret set ANDROID_KEY_ALIAS
  printf %s "$pass" | gh secret set ANDROID_KEY_PASSWORD
  echo "Os 4 secrets foram cadastrados no GitHub. A próxima release já sai com o APK assinado."
else
  openssl base64 -A -in "$KEY" > "$DIR/keystore-base64.txt"
  echo
  echo "Sem o GitHub CLI (gh). Cadastre à mão em GitHub → Settings → Secrets and"
  echo "variables → Actions → New repository secret:"
  echo "  ANDROID_KEYSTORE_BASE64   = conteúdo de $DIR/keystore-base64.txt"
  echo "  os outros 3               = os valores de $DIR/senhas.txt"
fi
echo
echo "IMPORTANTE: guarde uma cópia da pasta $DIR/ num lugar seguro (pendrive,"
echo "gerenciador de senhas). Ela não vai para o git."
