#!/usr/bin/env bash
# Publica o intermediário grátis (cmd/api) no Google Cloud Run.
# Prepara o projeto na primeira vez (APIs, repositório de imagens, segredo com
# a chave do Groq) e depois só publica a versão nova. Passo a passo e custos:
# docs/IA-GRATIS.md.
#
#   PROJECT=id-do-projeto bash scripts/deploy-proxy.sh
set -euo pipefail
cd "$(dirname "$0")/.."
: "${PROJECT:?defina PROJECT=<id do projeto no Google Cloud>}"
REGION=${REGION:-us-central1}
IMAGE="$REGION-docker.pkg.dev/$PROJECT/finan/finanproxy"
g() { gcloud --project "$PROJECT" "$@"; }

echo "→ Ligando as APIs necessárias (só demora na primeira vez)"
g services enable run.googleapis.com cloudbuild.googleapis.com artifactregistry.googleapis.com secretmanager.googleapis.com

if ! g artifacts repositories describe finan --location "$REGION" >/dev/null 2>&1; then
  echo "→ Criando o repositório de imagens"
  g artifacts repositories create finan --repository-format=docker --location "$REGION"
fi

if ! g secrets describe groq-api-key >/dev/null 2>&1; then
  echo "→ Cole a chave do Groq (não aparece na tela) e tecle Enter:"
  read -rs KEY
  [ -n "$KEY" ] || { echo "chave vazia" >&2; exit 1; }
  printf '%s' "$KEY" | g secrets create groq-api-key --data-file=-
  unset KEY
fi

# Conta só do intermediário: lê o segredo e nada mais. A conta padrão do
# Compute pode mexer no projeto inteiro; se o contêiner fosse tomado, iria junto.
SA="finanproxy@$PROJECT.iam.gserviceaccount.com"
if ! g iam service-accounts describe "$SA" >/dev/null 2>&1; then
  echo "→ Criando a conta de serviço do intermediário"
  g iam service-accounts create finanproxy --display-name "finanproxy (só lê a chave do Groq)"
fi
g secrets add-iam-policy-binding groq-api-key --member "serviceAccount:$SA" --role roles/secretmanager.secretAccessor >/dev/null

echo "→ Construindo a imagem"
g builds submit --config cmd/api/cloudbuild.yaml --substitutions "_IMAGE=$IMAGE" .

echo "→ Publicando (escala a zero, no máximo 1 instância)"
g run deploy finanproxy --image "$IMAGE" --region "$REGION" --allow-unauthenticated --service-account "$SA" \
  --min-instances 0 --max-instances 1 --concurrency 20 --memory 256Mi --cpu 1 --timeout 300 \
  --set-secrets GROQ_API_KEY=groq-api-key:latest

url=$(g run services describe finanproxy --region "$REGION" --format='value(status.url)')
echo
echo "Pronto: $url"
curl -fsS "$url/healthz" && echo "  (intermediário respondendo)"
echo "Coloque este endereço em FreeProxyURL (internal/llm/presets.go) para o app oferecer a opção grátis."
