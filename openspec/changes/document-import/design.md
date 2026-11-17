## Context

Reusar os importadores, pdf.js, categorias por histórico e gravação em lote existentes.
As mudanças anteriores e arquivos nativos pendentes são preservados. O trabalho fica neste
checkout, sem commit, push, merge ou deploy automático.

## Goals / Non-Goals

Suportar OFX/QFX, CSV, XLSX, PDF com texto ou páginas digitalizadas e imagens PNG/JPEG/WebP.
Arquivos ilegíveis, planilhas sem colunas financeiras e formatos legados não serão adivinhados.
Não incluir captura contínua de e-mail, senha bancária, automação do internet banking ou IA remota.
Recebimento nativo por extensões de compartilhamento exige uma etapa própria; o seletor de
arquivo do sistema é a entrada comum para web, iOS e Android nesta entrega.

## Decisions

XLSX usa ZIP/XML da biblioteca padrão Go, com limites de expansão e sem avaliar fórmulas.
OCR usa um worker local por importação e modelo de português. Imagens e documentos têm
limites de bytes, páginas, pixels e tempo. OCR não repara números automaticamente. A prévia
permite conferir tipo e campos; resultados por OCR começam desmarcados para confirmação.
Correspondências entre fontes são possíveis duplicações desmarcadas, nunca apagadas.
Dados essenciais e cálculos continuam em Go; insights não são gerados de texto não confirmado.

## Risks / Trade-offs

OCR aumenta o tamanho dos recursos; medir antes de aceitar, sem reduzir o gate de APK.
Dispositivos reais e modelos bancários não fornecidos continuam exigindo validação específica.
Texto extraído não implica interpretação financeira correta: falhar com orientação é melhor
que inventar uma data, sentido ou valor. O original permanece disponível para a pessoa conferir.
