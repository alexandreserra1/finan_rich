-- Quem do time respondeu (jade, bento, luna, tito, otavio). Vazio nas
-- mensagens antigas e nas perguntas: a tela mostra a Jade.
ALTER TABLE chat_messages ADD COLUMN agent TEXT NOT NULL DEFAULT '';
