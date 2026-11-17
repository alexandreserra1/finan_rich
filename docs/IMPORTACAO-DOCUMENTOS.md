# Importação de documentos financeiros

O Rich lê o arquivo no aparelho, prepara uma prévia e só grava os lançamentos depois
da confirmação. Os painéis e insights existentes usam esses registros confirmados.
Uma foto não confirma saldo bancário e não substitui uma conexão com o banco.

## O que pode entrar

| Formato | Leitura | Conferência necessária |
|---|---|---|
| OFX/QFX e CSV | Importadores Go existentes | Datas, sentido, conta e categorias |
| XLSX | ZIP/XML em Go, sem executar fórmulas | Somente a primeira aba financeira reconhecida |
| PDF com texto | pdf.js local e interpretação em Go | Layout, datas e colunas do banco |
| PDF digitalizado | Renderização local e OCR | Linhas começam desmarcadas |
| PNG, JPEG e WebP estático | OCR local em português | Linhas começam desmarcadas |

É possível escolher extrato, fatura ou comprovante quando o reconhecimento automático
não identifica o tipo. Extratos exigem datas completas e sentido identificável.
Comprovantes sem data, valor, direção ou confirmação suficientes são recusados.
Linhas de extrato com dois valores, como débito e saldo, exigem outra exportação;
o app não escolhe um deles por adivinhação.

Não há suporte universal: XLS antigo, ODS, DOCX, HEIC, fotos manuscritas, boletos sem
confirmação e layouts desconhecidos podem precisar de exportação CSV/OFX ou registro
manual. Uma fatura continua usando as regras de datas do importador existente;
confira o ano quando o documento não o informa em cada compra.

## Como usar

1. Abra **Importar** e escolha o tipo de documento ou o reconhecimento automático.
2. Escolha o arquivo no seletor do sistema. Acompanhe a leitura ou cancele.
3. Confira o texto extraído, quando disponível, e compare com o documento original.
4. Escolha a conta e revise data, descrição, valor, entrada/saída e categoria.
5. Marque somente as linhas corretas e confirme a importação.

A troca de conta preserva as correções. A prévia sinaliza registros já importados e
possíveis duplicações pelo contexto, não apenas pelo valor. Correspondências duvidosas
começam desmarcadas; compras legítimas iguais não são apagadas. A identificação de novas
importações considera a conta, preservando os identificadores antigos já existentes.

## Privacidade e limites

- O documento original, a imagem e o texto bruto não são enviados a OCR ou IA externos.
- O original não é salvo pelo importador. A prévia é descartada ao sair da sessão.
- Depois da confirmação, os lançamentos seguem a sincronização já configurada pela pessoa.
- A sugestão opcional de categorias por IA continua separada e envia as descrições
  selecionadas ao provedor configurado. Ela não é chamada pela leitura do documento.
- Arquivos: até 8 MiB. PDF: até 30 páginas, no máximo seis páginas com OCR.
- Imagens: até 20 milhões de pixels e 10.000 pixels por lado; animações são recusadas.
- OCR: até 45 segundos por imagem; a interface cancela a operação após 120 segundos.
- Texto: até 1 MiB. XLSX: até 16 MiB descompactados, 256 entradas ZIP, 32 abas,
  10.000 linhas, 128 colunas e 200.000 células.
- Fórmulas, macros, referências externas, datas inválidas e centavos imprecisos no XLSX
  são recusados. O arquivo inválido não é importado parcialmente.

O OCR usa Tesseract.js-core 6.1.2 e o modelo português rápido tessdata_fast 4.1.0,
Apache-2.0. Os recursos locais têm hashes conferidos em
`internal/web/static/vendor/ocr/resources.json`. No app eles acompanham a instalação.
No site, são baixados do próprio Rich no primeiro uso e guardados pelo service worker;
o primeiro uso offline sem esses recursos não funciona. Não há cobrança por página
nem conta de provedor OCR, mas a hospedagem continua tendo seus próprios limites.

## Verificação reproduzível

```sh
make test
make e2e
cd e2e && BROWSER=webkit npm run test:core
go test -race ./internal/importer ./internal/wasmapi ./internal/web
node --test scripts/document-assets.test.mjs
scripts/check-licenses.sh
node scripts/engineering.mjs check affected
```

O teste `e2e/documents.test.mjs` usa dados fictícios: planilha, PNG/JPEG/WebP quando
o navegador consegue codificá-lo, PDF digitalizado com rodapé textual, OCR real,
campos corrigidos, conta alterada, confirmação, reimportação entre fontes, cancelamento,
arquivo danificado e imagens abusivas. Ele recusa requisições HTTP para fora do servidor
local e registra erros do navegador. Capturas de 390, 768, 1440 e 1920 pixels, nos dois
temas, ficam em `.artifacts/document-import/`; o CI publica somente essas provas fictícias.

Os testes de domínio cobrem centavos, datas, expansão ZIP, fórmulas, conteúdo externo,
ambiguidade e duplicação. O histórico verificável dos gates atuais fica em
`.artifacts/engineering/verification.json`; uma alteração posterior invalida a evidência.

## O que ainda exige uma etapa própria

O Android agora recebe arquivos pelo menu **Compartilhar** de outros apps; detalhes em
[COMPARTILHAR-DOCUMENTOS.md](COMPARTILHAR-DOCUMENTOS.md). A extensão do iOS ainda não foi
implementada. O seletor de arquivos continua sendo a entrada comum em todas as plataformas.
A validação em Chromium e WebKit não substitui testes em iPhone e Android reais,
incluindo memória, permissões e fotos de bancos diferentes. Não há promessa de
reconhecer todos os layouts ou corrigir números ilegíveis automaticamente.

## Evidências locais de 16/11/2026

- `make test`: formatação, vet, testes Go e regras executadas como WebAssembly aprovados.
- `make e2e`: 17 scripts Chromium aprovados, incluindo a importação e o Google simulado.
- `BROWSER=webkit npm run test:core`: 16 scripts aprovados. OCR foi executado de verdade,
  mas o banco, o login e os documentos são de teste; isso não valida contas reais.
- A prévia foi inspecionada nas quatro larguras e nos dois temas. Campos cortados no
  celular e transparência que prejudicava a leitura foram corrigidos.
- Revisões independentes de código e segurança não mantiveram bloqueadores após as
  correções. Casos relevantes ganharam testes: débito com sinal e letra D, saldo na mesma
  linha do movimento, datas e centavos inválidos, cancelamento e PDF com imagem e rodapé.
- Testes com detector de concorrência passaram em importer, wasmapi e web. Importer e
  wasmapi foram repetidos após a correção dos marcadores de débito.
- 23 testes do controlador e dos recursos passaram. Licenças aprovadas;
  govulncheck e auditoria npm dos recursos não encontraram vulnerabilidades conhecidas.
- APK local de release compilado: **18.747.302 bytes**, abaixo do limite inalterado de
  **18.874.368 bytes**. Foi usada a assinatura de desenvolvimento local, não uma publicação.
  Imagens exclusivas da landing foram retiradas somente do pacote nativo; o site as mantém.
- Nenhum commit, push, merge, deploy ou acesso a banco de produção foi feito nesta entrega.

O resultado consolidado sobre os arquivos finais deve ser conferido no gate de engenharia
indicado acima. Os logs temporários desta execução usam o prefixo `/tmp/rich-document-`.
