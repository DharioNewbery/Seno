-- 0008_execucao_historico.sql — tela de ensaio com testes sempre visíveis e
-- histórico de ensaios anteriores: guarda o código submetido e a saída de
-- erro (stderr/compilador) por linha de execucao, para o GET do histórico
-- mostrar o código antigo rodado e mensagens de erro legíveis.
-- Idempotente (padrão da 0004): bancos vivos não contêm as colunas.

BEGIN;

ALTER TABLE execucao ADD COLUMN IF NOT EXISTS codigo TEXT;
ALTER TABLE execucao ADD COLUMN IF NOT EXISTS stderr TEXT;
ALTER TABLE execucao ADD COLUMN IF NOT EXISTS compilacao TEXT;

COMMIT;
