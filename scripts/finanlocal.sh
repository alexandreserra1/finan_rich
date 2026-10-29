#!/bin/sh
# Liga este Mac como provedor de IA do finan_rich (sem cota de tokens):
#   1. gera a chave em ~/.config/finan_rich/local.key (só você lê; fora do repositório);
#   2. compila o porteiro (cmd/finanlocal) em ~/.config/finan_rich/finanlocal;
#   3. deixa o Ollama (com contexto de 8 mil tokens) e o porteiro ligando sozinhos
#      quando você entra no Mac (launchd, em ~/Library/LaunchAgents);
#   4. abre o túnel do Tailscale Funnel para o porteiro e mostra o endereço;
#   5. copia a chave para a área de transferência (nunca mostra na tela).
# Depois: na Vercel, FINAN_LOCAL_URL = <endereço>/v1 e FINAN_LOCAL_KEY = (cole), e Redeploy.
#
#   scripts/finanlocal.sh          liga (pode rodar de novo: não troca a chave)
#   scripts/finanlocal.sh --off    desliga (tira o túnel e os serviços; a chave fica)
set -eu

DIR="$HOME/.config/finan_rich"
KEY="$DIR/local.key"
BIN="$DIR/finanlocal"
AGENTS="$HOME/Library/LaunchAgents"
MODEL="${FINAN_LOCAL_MODEL:-qwen2.5:7b-instruct}"
PORT=11435
# O Tailscale da App Store e o standalone não põem o comando no PATH; ele fica dentro do app.
TS=$(command -v tailscale || true)
[ -n "$TS" ] || [ ! -x /Applications/Tailscale.app/Contents/MacOS/Tailscale ] || TS=/Applications/Tailscale.app/Contents/MacOS/Tailscale

if [ "${1:-}" = "--off" ]; then
  [ -n "$TS" ] && "$TS" funnel --https=443 off 2>/dev/null || true
  for name in ollama local; do
    launchctl bootout "gui/$(id -u)/com.finanrich.$name" 2>/dev/null || true
    rm -f "$AGENTS/com.finanrich.$name.plist"
  done
  echo "Desligado. Tire FINAN_LOCAL_URL da Vercel para o intermediário não tentar mais este Mac."
  exit 0
fi

command -v ollama >/dev/null || { echo "Instale o Ollama primeiro: https://ollama.com/download"; exit 1; }
command -v go >/dev/null || { echo "Precisa do Go para compilar o porteiro: https://go.dev/dl"; exit 1; }
cd "$(dirname "$0")/.."

mkdir -p "$DIR" "$AGENTS"
chmod 700 "$DIR"
if [ ! -s "$KEY" ]; then
  umask 077
  openssl rand -base64 36 | tr -d '\n=/+' > "$KEY"
fi
chmod 600 "$KEY"

go build -trimpath -o "$BIN" ./cmd/finanlocal
ollama list | grep -q "^$MODEL" || ollama pull "$MODEL"

# plist escreve um serviço do launchd que liga ao entrar no Mac e volta se cair.
plist() { # nome, depois o programa e os argumentos
  name=$1; shift
  file="$AGENTS/com.finanrich.$name.plist"
  {
    echo '<?xml version="1.0" encoding="UTF-8"?>'
    echo '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">'
    echo '<plist version="1.0"><dict>'
    echo "<key>Label</key><string>com.finanrich.$name</string>"
    echo '<key>ProgramArguments</key><array>'
    for a in "$@"; do echo "<string>$a</string>"; done
    echo '</array>'
    echo '<key>EnvironmentVariables</key><dict><key>OLLAMA_CONTEXT_LENGTH</key><string>8192</string></dict>'
    echo '<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>'
    echo "<key>StandardErrorPath</key><string>$DIR/$name.log</string>"
    echo '</dict></plist>'
  } > "$file"
  launchctl bootout "gui/$(id -u)/com.finanrich.$name" 2>/dev/null || true
  launchctl bootstrap "gui/$(id -u)" "$file"
}

pkill -f "ollama serve" 2>/dev/null || true # o do launchd assume, com o contexto certo
plist ollama "$(command -v ollama)" serve
plist local "$BIN" -keyfile "$KEY" -model "$MODEL" -addr "127.0.0.1:$PORT"
sleep 3
curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:$PORT/v1/models" | grep -q 401 \
  && echo "Porteiro ligado em 127.0.0.1:$PORT (sem a chave, recusa)." \
  || { echo "O porteiro não respondeu; veja $DIR/local.log"; exit 1; }

pbcopy < "$KEY"
echo "A chave foi copiada para a área de transferência (está em $KEY; não mostre a ninguém)."

if [ -n "$TS" ]; then
  echo "Abrindo o túnel. Se o Tailscale mostrar um link para liberar o HTTPS e o Funnel, abra, aprove e espere."
  "$TS" funnel --bg "$PORT"
  host=$("$TS" status --json | sed -n 's/.*"DNSName": *"\([^"]*\)\.".*/\1/p' | head -1)
  echo
  echo "Na Vercel (projeto finan-rich-ia → Settings → Environment Variables):"
  echo "  FINAN_LOCAL_URL = https://$host/v1"
  echo "  FINAN_LOCAL_KEY = (cole: já está na área de transferência)"
  echo "Depois: Deployments → o último → Redeploy."
else
  echo
  echo "Falta o túnel: instale o Tailscale (https://tailscale.com/download/mac, pela App Store ou a versão"
  echo "standalone), entre com a sua conta, ligue o HTTPS e o Funnel quando ele pedir, e rode este script de novo."
fi
