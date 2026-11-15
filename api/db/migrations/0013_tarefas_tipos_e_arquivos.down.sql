BEGIN;

DROP TABLE IF EXISTS submissao_arquivos;

UPDATE atividade_tarefas SET linguagem = 'python' WHERE linguagem = '';

ALTER TABLE atividade_tarefas
    ALTER COLUMN linguagem DROP DEFAULT,
    DROP CONSTRAINT IF EXISTS atividade_tarefas_linguagem_check,
    ADD CONSTRAINT atividade_tarefas_linguagem_check
        CHECK (linguagem IN ('python', 'c', 'cpp'));

ALTER TABLE tarefas
    DROP COLUMN IF EXISTS config,
    DROP COLUMN IF EXISTS tipo;

COMMIT;
