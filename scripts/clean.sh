#!/usr/bin/env bash
# Limpa restos de build, caches e imagens Docker que não são mais usados.
#
# Por padrão só MOSTRA o que apagaria (e quanto espaço libera). Para apagar
# de verdade, acrescente --yes.
#
#   scripts/clean.sh                  # saídas de build do projeto (seguro)
#   scripts/clean.sh --deps           # + node_modules (o próximo build reinstala)
#   scripts/clean.sh --caches         # + caches do Go, npm e Gradle desta máquina
#   scripts/clean.sh --docker         # + imagens/camadas Docker sem uso
#   scripts/clean.sh --all --yes      # tudo isso, apagando
#
# NUNCA apaga dados: arquivos *.db, a pasta de dados do finan_rich, o .env, o
# keystore de assinatura nem volumes do Docker (o volume finan-data guarda o
# banco de quem usa o app pelo Docker).
set -euo pipefail
cd "$(dirname "$0")/.."

yes=0 deps=0 caches=0 docker=0
for arg in "$@"; do
  case "$arg" in
    --yes) yes=1 ;;
    --deps) deps=1 ;;
    --caches) caches=1 ;;
    --docker) docker=1 ;;
    --all) deps=1 caches=1 docker=1 ;;
    -h|--help) sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "opção desconhecida: $arg (veja --help)" >&2; exit 2 ;;
  esac
done

total=0
remove() { # remove <caminho>...: apaga (ou só lista) caminhos que existem
  for p in "$@"; do
    [ -e "$p" ] || continue
    case "$p" in *.db|*.db-wal|*.db-shm|*.jks|*.keystore|.env) echo "PULADO (dado) $p"; continue ;; esac
    kb=$(du -sk "$p" 2>/dev/null | cut -f1)
    total=$((total + kb))
    printf '%8s  %s\n' "$((kb / 1024)) MB" "$p"
    [ "$yes" = 1 ] && rm -rf -- "$p"
  done
  return 0
}
run() { # run <descrição> <comando...>: roda (ou só mostra) um comando de limpeza
  local what=$1; shift
  if [ "$yes" = 1 ]; then echo "→ $what"; "$@" || echo "  (falhou, seguindo)"; else echo "  rodaria: $*   # $what"; fi
}

echo "== Saídas de build do projeto =="
remove dist \
  internal/web/static/finance.wasm internal/web/static/js/wasm_exec.js \
  mobile/www \
  mobile/android/build mobile/android/app/build mobile/android/.gradle \
  mobile/android/capacitor-cordova-android-plugins/build \
  mobile/android/app/src/main/assets/public \
  mobile/ios/App/App/public mobile/ios/App/build mobile/ios/DerivedData \
  e2e/test-results e2e/playwright-report
# Arquivos soltos de testes e builds manuais (cobertura, binários de teste, APKs).
while IFS= read -r -d '' f; do remove "$f"; done < <(find . -path ./.git -prune -o -path '*/node_modules' -prune -o \
  -type f \( -name '*.test' -o -name 'coverage.out' -o -name '*.coverprofile' -o -name '*.apk' -o -name '*.aab' \) -print0)

if [ "$deps" = 1 ]; then
  echo "== Dependências instaladas (voltam no próximo npm ci) =="
  remove mobile/node_modules e2e/node_modules
fi

if [ "$caches" = 1 ]; then
  echo "== Caches desta máquina =="
  command -v go >/dev/null && run "cache de build e de testes do Go" go clean -cache -testcache
  command -v npm >/dev/null && run "cache do npm" npm cache clean --force
  remove "$HOME/.gradle/caches/build-cache-1" "$HOME/.gradle/daemon"
fi

if [ "$docker" = 1 ]; then
  echo "== Docker (sem volumes: os dados ficam) =="
  if command -v docker >/dev/null && docker info >/dev/null 2>&1; then
    docker system df || true
    run "contêineres parados" docker container prune -f
    run "imagens sem tag (restos de builds antigos)" docker image prune -f
    run "cache de build" docker builder prune -f
  else
    echo "  Docker não está rodando; nada a fazer."
  fi
fi

echo
if [ "$yes" = 1 ]; then
  echo "Liberado: $((total / 1024)) MB (fora caches do Go/npm e Docker)."
else
  echo "Total: $((total / 1024)) MB. Nada foi apagado; rode de novo com --yes para apagar."
fi
