## Context

Android usa ACTION_SEND com EXTRA_STREAM e concessão temporária de leitura de content URI.
O plugin Share já instalado serve para enviar, não para receber. Reutilizar os importadores
Go/Wasm e a revisão existente, sem copiar o documento para disco ou enviar à internet.

## Goals / Non-Goals

Receber um arquivo por vez no Android, com o app aberto ou iniciando. Não coletar texto
solto, links, pastas ou vários arquivos. Não alterar autenticação, financeiro ou bancos.
Não implementar uma extensão iOS sem poder compilar e testar sua integração.

## Decisions

SharedDocuments mantém uma pendência por instância, com identificador aleatório e validade
de dez minutos. peek retorna somente metadados; read e discard exigem o identificador.
A chegada de outro envio não sobrescreve o anterior. O evento pending não transporta dados.
Somente content URI externo é aceito; recusar provedores do próprio UID evita que um app
externo faça a Rich ler seus próprios arquivos privados. Não pedir acesso amplo ao armazenamento.

O app mostra um aviso sem navegar automaticamente ou redesenhar formulários em edição.
Ao tocar em revisar, abre Importar; uma prévia existente só é substituída após confirmação.
O bloqueio do app impede leitura e revisão. O mesmo limite de 8 MiB é imposto na leitura
nativa e na ponte. A gravação financeira permanece separada e exige confirmação.

## Risks / Trade-offs

O sistema pode encerrar o processo e perder a pendência: nesse caso, a pessoa compartilha
novamente. Não persistir conteúdo evita criar um segundo armazenamento sensível.
MIME e nomes são informativos; a interpretação confere o conteúdo real. Falhas de concessão
ou provedores indisponíveis não devem salvar dados nem expor detalhes internos.
Uma validação em emulador não substitui provedores de bancos e celulares reais.
