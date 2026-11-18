-- Registro de acesso (Ajustes → Acesso pelo celular): entradas pela rede, senhas
-- erradas, pareamentos, troca de senha, backup baixado. Sem senha, token nem dado
-- financeiro: só o que aconteceu, quando e de que endereço.
CREATE TABLE access_log (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    event TEXT NOT NULL,
    ip    TEXT NOT NULL DEFAULT ''
);
