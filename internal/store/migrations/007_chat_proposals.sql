-- Mudanças propostas pelo consultor numa resposta (lançar, alterar, apagar,
-- criar meta ou dívida), em JSON: os cartões continuam na conversa depois de
-- recarregar a página.
ALTER TABLE chat_messages ADD COLUMN proposals TEXT NOT NULL DEFAULT '';
