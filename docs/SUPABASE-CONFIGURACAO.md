# Preparar o Supabase da Rich

O Supabase Auth será o cadastro por e-mail e senha. O login Google continua
opcional. Criar o projeto não muda os dados atuais, não transfere o Drive e não
ativa o cadastro na Rich: a integração ainda precisa concluir os bloqueios de
[CADASTRO-SEM-GOOGLE.md](CADASTRO-SEM-GOOGLE.md).

## 1. Criar o projeto

1. Abra [o painel Supabase](https://supabase.com/dashboard) e entre na sua conta.
2. Crie ou selecione uma organização e clique em **New project**.
3. Nome: `rich`. Gere uma senha longa para o banco e guarde num gerenciador de
   senhas. Não envie essa senha no chat nem salve no Git.
4. Escolha uma região próxima das pessoas que usarão o aplicativo, entre as
   disponíveis. Não contrate plano pago sem revisar limites e custos.
5. Aguarde o projeto ficar pronto.

## 2. Guardar a identificação, sem publicar segredos

Nos ajustes de API/conexão do projeto, identifique:

- **Project URL**: endereço HTTPS do projeto.
- **Publishable key**: chave pública com prefixo `sb_publishable_`, aceita pela
  base do adaptador desta entrega. Ela não substitui uma sessão de usuário.
- Acesso PostgreSQL: será configurado no backend com papéis restritos; não usar
  a senha administrativa como credencial do aplicativo.

Não copie `service_role`, secret key, senha do banco ou string de conexão para
JavaScript público, screenshots, chat ou PR. Ainda não existem variáveis de
ativação na Rich: não inventar nomes ou colar chaves na Vercel antes de ligar o BFF.

## 3. Autenticação e e-mail

Em **Authentication**, mantenha o provedor **Email** e a confirmação de e-mail
ativados. A Rich não deve abrir uma conta sem confirmação.

Antes de receber clientes reais, configure SMTP próprio e verifique o domínio
remetente. O serviço padrão do Supabase é restrito e não serve como entrega de
e-mail de produção para clientes. Escolher o serviço, domínio e orçamento faz
parte da ativação; nenhum foi contratado automaticamente.

A senha de cadastro terá pelo menos 15 caracteres e no máximo 72 bytes, sem
truncamento. Espaços e Unicode são aceitos dentro desse limite. O limite em bytes
vem do algoritmo atual do provedor; a tela precisa explicá-lo quando necessário.
Configure a política do provedor de modo compatível e verifique proteção contra
senhas comprometidas e abuso antes de abrir o cadastro.

Os endereços de confirmação e recuperação serão cadastrados quando as rotas do
BFF existirem. Não usar wildcard de preview em produção nem links de um domínio
informado livremente pelo navegador. O endereço principal deve ser o domínio de
produção da Rich.

## 4. O que não executar ainda

Não aplique a migração SQL no projeto real enquanto faltarem revisão final,
sessões do BFF, cifra dos tokens, revogação e isolamento do banco local. O teste
`sh scripts/test-identity-db.sh` cria apenas um PostgreSQL descartável no Docker,
sem rede, sem dados pessoais e sem conexão com seu Supabase.

O cadastro só será habilitado depois de testar confirmação, recuperação, CSRF,
limites de abuso, duas contas e duas abas. Antes disso, nenhum botão deve prometer
“criar conta” e simular que uma conta foi criada.

Referências: [senhas](https://supabase.com/docs/guides/auth/passwords),
[SMTP](https://supabase.com/docs/guides/auth/auth-smtp),
[segurança de produção](https://supabase.com/docs/guides/security/production-checklist).
