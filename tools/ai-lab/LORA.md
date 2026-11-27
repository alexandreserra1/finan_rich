# Preparação do piloto LoRA no Mac

**LoRA continua pausado em 27/11/2026.** A quarta revisão independente reprovou o contexto
9f32c90c: `blocked`, `parallel_agents`, 18 bloqueantes e 34 avisos, incluindo repetições.
Correções e pacote para a quinta rodada não liberam treino. As provas abaixo são históricas;
não autorizam downloads de pesos/tokenizers, treino, publicação ou upload de dados.
Retomar somente após erro residual demonstrado e nova aprovação explícita da pessoa.

O extrator agora aceita conversas multivolta e gera um alvo por turno do assistente,
incluindo chamadas sucessivas bem-sucedidas. Uma chamada rejeitada pelo executor fica
somente no contexto da recuperação, nunca como alvo positivo a imitar. Se um turno mistura
chamadas aceitas e rejeitadas, ele não vira alvo; a conversa original não é reescrita.
Oferecer ferramenta sem chamá-la não transforma a resposta
em alvo de chamada. As contagens de 40 alvos abaixo pertencem ao corpus antigo; nenhum
pacote novo foi preparado a partir dos 70 cenários do gerador refeito.

Estado: alvos supervisionados e preflight implementados. **O Qwen não foi treinado nesta etapa.**
O corpus financeiro continua com revisão humana pendente e `training_allowed: false`.
O [orquestrador](ORQUESTRACAO.md) encontrou bloqueios financeiros no contexto do corpus;
preflight de tokenizer/Metal aprovado não remove esses bloqueios.

## Problemas encontrados antes de treinar

O `ChatDataset` do MLX LM 0.32.2 mascara tudo antes da última mensagem do assistente.
Uma conversa contendo chamada, resultado da ferramenta e resposta final ensinaria somente
a resposta final. A chamada intermediária ficaria fora da perda.

`prepare_lora.py` resolve isso sem inventar respostas: cada conversa com ferramenta gera
dois alvos, um terminando na chamada e outro na resposta final. As mensagens de usuário,
sistema e resultado da ferramenta ficam no prefixo mascarado. As famílias não mudam de
conjunto. São **40 alvos derivados dos 24 exemplos**, não 40 cenários novos: 28/6/6.

Também reproduzimos desalinhamento entre prefixo e conversa completa no template padrão
do Qwen3.5. `NoThinkingTokenizer` força `enable_thinking=False` nos dois caminhos; o
preflight exige igualdade exata dos tokens do prefixo e rejeita alvo vazio ou truncado.
Não usar o CLI MLX sem essa adaptação e presumir que a máscara está correta.

## Ambiente isolado

Foi criado `.artifacts/ai-lab/mlx-venv`, com Python 3.11, fora do app e do ambiente padrão
dos testes sem modelos. MLX LM 0.32.2, MLX 0.32.5 e Transformers 5.24.1 foram instalados.
As versões diretas e transitivas estão em `requirements-mlx.lock`; `pip check` passou.
Esse arquivo fixa versões, mas não contém hashes de cada wheel nem é uma auditoria de segurança.

Os testes offline continuam com Python >=3.9 e sem esses pacotes. CI não instala MLX,
não baixa modelos e não roda o smoke Metal automaticamente.

Reprodução do ambiente experimental em macOS ARM64:

```sh
python3.11 -m venv .artifacts/ai-lab/mlx-venv
.artifacts/ai-lab/mlx-venv/bin/python -m pip install --only-binary=:all: -r tools/ai-lab/requirements-mlx.lock
```

Licenças foram inventariadas, não liberadas automaticamente. Há expressões compostas
em NumPy e regex, além de PSF-2.0 em typing_extensions, que exigem conferência da política
do projeto antes de adotar esse runtime. Não se trata de API paga. O resultado do checker
de licenças do produto não cobre essas dependências Python do laboratório. Não houve
mudança na allowlist nem inclusão dessas bibliotecas no binário, site ou aplicativo.

## Metadados do modelo, sem pesos de treinamento

Somente config, tokenizer, template e licença foram obtidos do repositório oficial
`Qwen/Qwen3.5-4B`, revisão `851bf6e806efd8d0a36b00ddf55e13ccb7b8cd0a`.
`runtime-model.json` registra hashes. Não é a distribuição GGUF usada pelo Ollama.
Não baixamos o checkpoint Transformers completo nem carregamos pesos Qwen no MLX.

Para repor esses arquivos públicos, usando o ambiente experimental:

```sh
HF_HUB_DISABLE_TELEMETRY=1 HF_HUB_DISABLE_IMPLICIT_TOKEN=1 .artifacts/ai-lab/mlx-venv/bin/python - <<'PY'
import json
from pathlib import Path
from huggingface_hub import snapshot_download
reference = json.loads(Path('tools/ai-lab/runtime-model.json').read_text())
snapshot_download(reference['repository'], revision=reference['revision'], token=False,
                  allow_patterns=list(reference['files']),
                  local_dir='.artifacts/ai-lab/qwen35-tokenizer', max_workers=2)
PY
```

O preflight usa somente cópias dos bytes verificados num diretório temporário exclusivo.
Templates extras, inclusive `additional_chat_templates/tool_use.jinja`, não entram nessa
cópia. Isso evita que o Transformers escolha conteúdo não coberto pelos hashes. Nenhum
código remoto é habilitado e a tokenização usa `local_files_only=True`.

## Executar preparação e verificação

Troque `CAMINHO_DO_CORPUS` pela pasta impressa por `prepare_dataset.py` e `CAMINHO_LORA`
pela pasta impressa por `prepare_lora.py`. Não são valores finais de configuração.

```sh
PYTHONDONTWRITEBYTECODE=1 .artifacts/ai-lab/venv/bin/python tools/ai-lab/prepare_lora.py CAMINHO_DO_CORPUS
HF_HUB_OFFLINE=1 HF_HUB_DISABLE_TELEMETRY=1 TOKENIZERS_PARALLELISM=false PYTHONDONTWRITEBYTECODE=1 .artifacts/ai-lab/mlx-venv/bin/python tools/ai-lab/probe_mlx.py CAMINHO_LORA
```

O preparador rejeita links/arquivos excessivos, hashes divergentes, conjuntos que não
correspondem à auditoria e famílias cruzadas. O probe reconfere fonte e alvos antes/depois,
exige versões fixadas e não transforma revisão pendente em aprovação.

O processo normal do probe tem deadline externo de 120 segundos e relatório `blocked`
em timeout. Um diretório interrompido sem relatório final não é uma verificação válida.
Se o worker for encerrado à força, a cópia temporária dos arquivos públicos pode sobrar
em `.artifacts`; isso não libera treinamento nem entra no repositório.

## Smoke numérico e evidências

Além da tokenização, foi executado LoRA em uma **camada aleatória 8x8**, com rank 2 e
três passos SGD, sob limite de memória MLX de 256 MiB. A perda caiu de aproximadamente
0,466622 para 0,466341; a camada-base ficou intacta e os gradientes ficaram nos adaptadores.
Não foram exportados adaptadores. Isso verifica o backend de diferenciação no Metal;
não demonstra qualidade financeira nem viabilidade de treinar o modelo de 4 bilhões.

O preflight observou 40 prefixos alinhados, entre 3.114 e 4.123 tokens, sem truncamento.
Um limite de sequência 4.096 já cortaria parte dos exemplos. O teto de 8.192 do preflight
é uma barreira contra entrada excessiva, não aprovação desse contexto para treino em 16 GiB.

Relatórios ficam em `.artifacts/ai-lab/mlx-probe-*/report.json`; registram revisão do
tokenizer, hash do manifesto, versões, máscara, contagens, smoke e inventário de licenças.
O pico medido no smoke é de arrays MLX pequenos, não da RAM total nem do Qwen.

Verificação desta etapa: `.artifacts/ai-lab/mlx-probe-v44rhfaz/report.json`, conjunto
`.artifacts/ai-lab/lora-data-5ztvlji3/`. Passaram 34 testes offline, incluindo templates
extras, hash adulterado, máscaras e deadline. A revisão técnica aprovou as correções;
aprovação financeira e de licenças continuam pendentes.

## O que falta para o piloto do modelo completo

Após corrigir o corpus financeiro, o conjunto derivado foi regenerado em
`.artifacts/ai-lab/lora-data-da8x7_d0/`, com 28/6/6 alvos. O novo preflight offline em
`.artifacts/ai-lab/mlx-probe-ytkm95ff/report.json` verificou 40 prefixos alinhados,
entre 3.187 e 4.403 tokens, sem truncamento. O smoke repetiu três passos somente na
camada aleatória 8x8; nenhum peso Qwen foi carregado nem adaptador exportado.
O crescimento do contexto reforça que 4.096 tokens não bastam. A revisão técnica
sequencial do corpus não libera treinamento; as pendências seguintes continuam válidas.

1. Definir quem revisa o conteúdo financeiro e registrar revisão por ID/hash; ampliar cobertura.
2. Concluir a conferência das licenças do runtime, sem enfraquecer a política do projeto.
3. Aprovar checkpoint convertido, revisão e licença; não reutilizar GGUF como se fosse safetensors.
4. Medir memória/tempo do Qwen real, começando com passos limitados e sem truncar alvos.
5. Implementar o treinador com revisão verificável, limites, checkpoints isolados e sem upload.
6. Comparar contra base e base+RAG em conjunto reservado; não liberar pelo valor da perda de treino.

Não há treinador financeiro nem promoção automática em produção nesta entrega.

## Fontes técnicas

- [Máscara no MLX LM 0.32.2](https://github.com/ml-explore/mlx-lm/blob/v0.32.2/mlx_lm/tuner/datasets.py).
- [Implementação LoRA usada no smoke](https://github.com/ml-explore/mlx-lm/blob/v0.32.2/mlx_lm/tuner/lora.py).
- [Guia oficial de LoRA](https://github.com/ml-explore/mlx-lm/blob/v0.32.2/mlx_lm/LORA.md).
- [Modelo oficial](https://huggingface.co/Qwen/Qwen3.5-4B/tree/851bf6e806efd8d0a36b00ddf55e13ccb7b8cd0a).
