#!/bin/sh
# Liga o graphify (mapa do código) ao ai-memory (memória longa que o Claude e o Codex
# consultam): atualiza o grafo e grava o resumo como a página concepts/mapa-do-codigo.
#
# Tudo fica neste computador: o "graphify update" relê só o código, sem IA, e o ai-memory
# roda aqui. graphify-out/ está no .gitignore. A primeira montagem, com docs e imagens,
# é o /graphify no Claude Code (pode usar IA); depois este script mantém o grafo em dia.
set -eu
cd "$(git rev-parse --show-toplevel)"
export PATH="$HOME/.local/bin:$PATH"

for tool in graphify ai-memory; do
  command -v "$tool" >/dev/null || { echo "falta o $tool (graphify: uv tool install graphifyy)" >&2; exit 1; }
done
if [ ! -f graphify-out/graph.json ]; then
  echo "ainda não há grafo: rode /graphify no Claude Code uma vez" >&2
  exit 1
fi

graphify update .

# O escopo é o do .ai-memory.toml, o mesmo para o Claude e o Codex.
workspace=$(sed -n 's/^workspace *= *"\(.*\)"/\1/p' .ai-memory.toml)
project=$(sed -n 's/^project *= *"\(.*\)"/\1/p' .ai-memory.toml)
header="Gerado por scripts/graphify-memoria.sh em $(date +%Y-%m-%d), commit $(git rev-parse --short HEAD). Para perguntas de estrutura (quem chama quem, o que liga duas partes), use o MCP graphify: query_graph, get_neighbors, shortest_path, god_nodes."
# Cada gravação troca a página inteira: o mapa antigo não fica misturado com o novo.
ai-memory write-page --workspace "$workspace" --project "$project" \
  --path concepts/mapa-do-codigo --kind fact --tier semantic \
  --tag graphify --tag arquitetura \
  --body "# Mapa do código (graphify)

$header

$(sed '1{/^# /d;}' graphify-out/GRAPH_REPORT.md)"
echo "Mapa do código atualizado no ai-memory (concepts/mapa-do-codigo)."
