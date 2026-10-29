-- Os números do app mostrados antes da resposta da IA (cartão "Os números do
-- app"). Vazio nas perguntas, nas respostas antigas e quando não houve conta.
ALTER TABLE chat_messages ADD COLUMN numbers TEXT NOT NULL DEFAULT '';
