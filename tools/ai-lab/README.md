# Laboratório local da IA Rich

**Estado em 26/11/2026:** corpus `blocked` após terceira revisão independente
(`parallel_agents`, 11 bloqueantes e 40 avisos). O resultado sequencial anterior foi
substituído. LoRA pausado; não baixar pesos, treinar, publicar modelo ou subir dados.
O formulário humano antigo está obsoleto e deve ser preservado. Codex mantém laboratório;
Claude corrige app e revisa PRs. A regeneração final aguarda aviso após correções na main.

Compara `qwen2.5:7b-instruct` já instalado com o candidato `qwen3.5:4b`, sem mudar
configurações do aplicativo. Usa Python padrão para orquestrar e o consultor Go real
para as perguntas financeiras. O controlador básico não depende de Transformers nem
PyTorch. O preflight MLX usa ambiente experimental separado, ainda não aprovado para treino.

Primeiro piloto executado: [resultados e limitações](RESULTADOS.md).
Preparação supervisionada: [corpus fictício e separação dos conjuntos](DATASET.md).
Piloto LoRA: [alvos, máscara e verificação no Mac](LORA.md).
Antes de liberar: [orquestrador OpenSpec e pareceres do corpus](ORQUESTRACAO.md).
Preparativo sem inferência: [avaliação reservada, app real e replay](AVALIACAO.md).

## Preparação

Requisitos: macOS ou Linux, Python >=3.9 com venv, Go na versão do projeto, Ollama local com os dois
modelos e espaço disponível. O serviço precisa escutar somente conforme a configuração
local; o laboratório chama exclusivamente `http://127.0.0.1:11434`.

```sh
python3 -m venv .artifacts/ai-lab/venv
make wasm
PYTHONDONTWRITEBYTECODE=1 .artifacts/ai-lab/venv/bin/python -m unittest discover -s tools/ai-lab -v
PYTHONDONTWRITEBYTECODE=1 .artifacts/ai-lab/venv/bin/python tools/ai-lab/evaluate.py
```

No ambiente desta entrega, o modelo de referência já existia. Em outra máquina,
`ollama pull qwen2.5:7b-instruct` é um download adicional de vários gigabytes, não
executado automaticamente pelo avaliador. Tags podem mudar: o script exige os digests
registrados em `models.json`. Revise origem e licença antes de atualizar esse manifesto.
Não altera o provedor salvo, não abre porta nova, não envia dados a uma API externa.

## O que executa

1. Registra versão do Ollama e hashes das fontes Go internas, dependências e manifesto do laboratório.
2. Confere modelos instalados, digests e presença da licença Apache nos metadados.
3. Executa uma pergunta textual por modelo com streaming, contexto 8192, seed 42,
   temperatura 0,7 e limite de 160 tokens. Registra primeiro texto, duração, tokens e
   tamanho residente observado pelo Ollama; **não mede pico de RAM do processo**.
4. Executa `TestEval` existente no pacote advisor, em Go, com livro financeiro fictício.
   Filtra sete casos: economia, mínimo do cartão, gasto com Uber, simulação de dívida,
   prazo da meta, comparação de gastos e próximos vencimentos.
5. Usa configuração explícita local e remove variáveis `FINAN_EVAL_*` herdadas, evitando
   trocar acidentalmente o provedor ou selecionar um conjunto vazio.
6. Reprova critérios incompletos mesmo quando `go test` retorna zero: o avaliador
   existente imprime notas, mas nem toda falha de qualidade gera erro do processo Go.
7. Descarrega os modelos do experimento antes e depois de cada avaliação, inclusive em falha;
   confere novamente os digests e os hashes das fontes antes de fechar o relatório.

O descarregamento pode ser assíncrono no Ollama. O laboratório aguarda até cinco segundos
pela saída de `/api/ps`; se a limpeza falhar, não inicia a próxima medição.

`TestEval` mantém seus parâmetros de amostragem, ferramentas e conhecimento existentes.
A seed do smoke test não se aplica ao TestEval. O tempo da avaliação inclui preparação
do comando Go e múltiplas chamadas; não é uma medição isolada de inferência.

## Evidências e limites

Cada execução cria um diretório exclusivo `.artifacts/ai-lab/run-*/`, ignorado pelo Git:

- `report.json`: metadados, smoke, notas, erros e estado final.
- `qwen2.5-7b-instruct.jsonl` e `qwen3.5-4b.jsonl`: eventos e respostas do TestEval.

O ambiente fica em `.artifacts/ai-lab/venv/`, sem pacotes externos. Um lock impede dois
laboratórios simultâneos de descarregarem os modelos um do outro. Não impede outro
programa de usar o Ollama; não execute trabalhos concorrentes durante a medição.

O comando retorna 1 se um candidato falhar, for pulado, ficar sem nota ou tiver critérios
incompletos. Uma falha não autoriza alterar a rubrica para aprovar o modelo. Relatórios
parciais permanecem com estado `running` caso a sessão seja interrompida; não são sucesso.

Cada avaliação Go tem timeout de cinco minutos e o processo é encerrado após 330 segundos.
O smoke limita tokens e linhas de streaming a 64 KiB, usa timeout de leitura de 90 segundos
e um subprocesso encerrado após 95 segundos, mesmo sem receber uma linha completa.
Não há repetição automática do laboratório nem treinamento em segundo plano.
Um erro de inicialização (como Ollama desligado) encerra o comando antes de gerar um
relatório novo; confira a data da evidência, não reutilize uma execução antiga como atual.

Os testes Python não exigem rede ou modelo e incluem casos de falsa aprovação e skip.
O gate `tooling` e o job de engenharia existente no CI executam esses testes por meio
de `scripts/ai-lab.test.mjs`; alterações em `tools/ai-lab/` acionam esse gate. O teste fica
separado dos testes de instalação das skills para manter os PRs independentes.
Os testes com modelos são opt-in; não entram no CI comum nem consomem cotas de nuvem.
Não introduzir dados pessoais nesses testes: os logs guardam respostas e contexto fictício.

Uma execução é um piloto, não benchmark estatístico nem aprovação financeira humana.
Não mede RAG, treinamento, classificação em produção, acesso concorrente ou iPhone.
Repetições, casos adversariais, fontes rotuladas e metas de latência/memória ainda precisam
ser definidos antes da adoção. Nenhum modelo é escolhido automaticamente para o app.

Arquitetura e próximas tarefas: [plano da IA especializada](../../openspec/changes/specialized-financial-ai/design.md).

## Fontes

- [Distribuição Ollama Qwen3.5-4B](https://ollama.com/library/qwen3.5:4b).
- [Modelo-base oficial no Hugging Face](https://huggingface.co/Qwen/Qwen3.5-4B).
- [API de streaming e métricas do Ollama](https://docs.ollama.com/api/generate).

O runtime é Ollama; não foi baixado o checkpoint Transformers diretamente do Hugging Face.
O experimento mantém o caminho já suportado pelo projeto e registra a distribuição testada.
