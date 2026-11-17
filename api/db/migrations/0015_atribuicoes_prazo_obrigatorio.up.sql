-- 0015_atribuicoes_prazo_obrigatorio.up.sql — Torna prazo obrigatório em atribuicoes.
BEGIN;

UPDATE atribuicoes a
SET prazo = COALESCE(
    (SELECT t.encerrado_em FROM turmas t WHERE t.id = a.turma_id),
    a.created_at + INTERVAL '30 days'
)
WHERE a.prazo IS NULL;

ALTER TABLE atribuicoes
    ALTER COLUMN prazo SET NOT NULL;

COMMIT;
