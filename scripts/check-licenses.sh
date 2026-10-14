#!/usr/bin/env bash
# Confere que tudo o que vai dentro do programa e do app é código aberto:
# - módulos Go compilados no binário (pela licença de cada um);
# - pacotes npm que vão no APK/iOS (Capacitor e plugins);
# - bibliotecas JavaScript copiadas em internal/web/static/vendor.
# Uma dependência com licença fora da lista faz o CI falhar.
set -euo pipefail
cd "$(dirname "$0")/.."
fail=0

echo "== Módulos Go =="
modcache=$(go env GOMODCACHE)
mods=$( (go list -deps -f '{{if .Module}}{{.Module.Path}}@{{.Module.Version}}{{end}}' ./cmd/finanrich
  GOOS=js GOARCH=wasm go list -deps -f '{{if .Module}}{{.Module.Path}}@{{.Module.Version}}{{end}}' ./cmd/finanwasm) |
  sort -u | grep -v '^github.com/alexandreserra1/')
for m in $mods; do
  path=${m%@*}
  dir="$modcache/$(echo "$path" | sed 's/[A-Z]/!\L&/g')@${m#*@}"
  file=$(ls "$dir"/LICENSE* "$dir"/COPYING* 2>/dev/null | head -1 || true)
  text=$(cat "$file" 2>/dev/null || true)
  case "$text" in
    *"Permission is hereby granted, free of charge"*) lic=MIT ;;
    *"Apache License"*) lic=Apache-2.0 ;;
    *"Redistribution and use in source and binary forms"*) lic=BSD ;;
    *"Mozilla Public License"*) lic=MPL-2.0 ;;
    *) lic="" ;;
  esac
  if [ -z "$lic" ]; then echo "FALHA $m: licença não reconhecida"; fail=1; else echo "ok    $m ($lic)"; fi
done

echo "== Pacotes do app (npm) =="
(cd mobile && npx --yes license-checker-rseidelsohn@4 --production --excludePrivatePackages \
  --onlyAllow 'MIT;ISC;0BSD;BSD-2-Clause;BSD-3-Clause;Apache-2.0' --summary) || fail=1

echo "== Bibliotecas em static/vendor =="
for d in internal/web/static/vendor/*/; do
  if grep -qE "Apache License|Permission is hereby granted|Redistribution and use" "$d"LICENSE* 2>/dev/null; then
    echo "ok    $d"
  else
    echo "FALHA $d: sem arquivo LICENSE de código aberto"; fail=1
  fi
done

exit $fail
