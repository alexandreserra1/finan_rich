#!/usr/bin/env bash
# Build do site na Vercel (projeto com Root Directory = site): gera as telas do
# app em modo site em site/www. A máquina de build da Vercel não traz Go para
# sites estáticos: baixamos a versão oficial e conferimos o sha256.
set -euo pipefail
cd "$(dirname "$0")/.."

if ! command -v go >/dev/null 2>&1; then
  version=1.26.10
  sum=ccfe7ae822d2c2f41f4fe7c956199967a4dac22d0ee0d2b2bfdb440aab3b20c5
  file="go$version.linux-amd64.tar.gz"
  curl -fsSL -o "/tmp/$file" "https://go.dev/dl/$file"
  echo "$sum  /tmp/$file" | sha256sum -c -
  tar -C /tmp -xzf "/tmp/$file"
  export PATH="/tmp/go/bin:$PATH"
fi

export GOFLAGS=-buildvcs=false # o clone da Vercel não deixa o Go ler o git
go run ./tools/wasmbuild
go run ./tools/mobilebuild -site site/www
