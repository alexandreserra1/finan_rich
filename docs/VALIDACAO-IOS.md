# Validação iOS sem Xcode neste Mac

O Swift local compila programas macOS, mas não substitui os SDKs de iPhone. Para não
depender dessa instalação, a validação nativa usa o job iOS já existente no GitHub Actions.
O runner macOS tem Xcode; não é necessário cadastrar conta Apple ou certificado para
compilar e testar no simulador. Isso não publica um aplicativo na loja.

## O que o PR solicita

1. Gerar as telas e o Wasm pelo mesmo caminho do app atual.
2. Sincronizar os plugins Capacitor no runner, sem substituir arquivos locais da pessoa.
3. Compilar o scheme compartilhado App para iPhoneSimulator, sem assinatura de distribuição.
4. Selecionar um iPhone disponível e executar AppUITests.
5. Exigir abertura do WebView e o aviso de aparelho novo; tocar em Conectar agora e
   conferir o botão Usar só neste celular nos Ajustes.
6. Guardar logs e capturas anexadas em `rich-ios-validacao` por sete dias.

O resultado completo fica em `resultado.xcresult`, que pode ser aberto depois em um Mac
com Xcode. O teste não faz login, não usa conta bancária, não grava dados reais e não
habilita integração externa. Uma captura não equivale a aprovação visual automática.

## Segurança da publicação desta branch

A autorização recebida vale somente para enviar `codex/validacao-ios-sem-xcode` e abrir
um PR. Não autoriza merge, release, tags ou deploy. O `vercel.json` desativa auto-deploy
somente nessa branch; a configuração das outras branches e de produção não é alterada.

Os três arquivos nativos que já estavam alterados antes deste trabalho permanecem fora
dos commits, assim como `docs/rich-mais-recente/`. Arquivos gerados, bancos, chaves e
credenciais também não entram. A identidade dos novos commits usa o perfil GitHub,
não o nome genérico deste Mac.

## Como interpretar o resultado

- **Local aprovado:** sintaxe do projeto/XML/Swift, seletor de simulador, contratos e gates
  disponíveis. O parser Swift local não resolve nem compila XCTest para iOS.
- **WebKit aprovado:** perfis de iPhone SE, 13 e 16 Pro Max foram usados na importação,
  confirmação e painel, incluindo temas e orientação. Não são aparelhos nem iOS nativo.
- **Remoto pendente:** até o job concluir, build e teste nativo não contam como aprovados.
- **Remoto aprovado:** apenas o fluxo coberto pelo XCUITest no simulador executado foi validado.
- **Aparelho físico pendente:** câmera, biometria, permissões, memória, bancos reais e
  versões diferentes de iOS precisam de validação própria.

Se não houver iPhone elegível, o seletor falha. Se build ou teste falhar, o código de saída
é preservado e o job reprova. Não há fallback para declarar sucesso sem executar.
O timeout existente de trinta minutos continua valendo.

A extensão Compartilhar do iOS ainda não foi implementada. Este PR cria a base verificável
para avançar nela; não deve ser apresentado como essa funcionalidade pronta.

Fontes: [runners GitHub](https://docs.github.com/en/actions/concepts/runners/github-hosted-runners),
[SDKs Apple](https://developer.apple.com/xcode/system-requirements/),
[deploy por branch Vercel](https://vercel.com/docs/project-configuration/git-configuration).
