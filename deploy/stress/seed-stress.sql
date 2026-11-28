-- deploy/stress/seed-stress.sql — Massa de dados idempotente para teste de estresse.
-- Cria (ou reseta apenas o escopo de estresse):
--   • Professor dono da turma: prof.stress@seno.dev (mesma senha do super@seno.dev, padrão: 102938)
--   • N alunos ativos (padrão 100): stress.aluno001@seno.dev .. stress.aluno100@seno.dev
--   • Matéria STRESS100, período (4, 2029) e turma T-STRESS-100 com todos os alunos matriculados.
--
-- As tarefas, a atividade e a atribuição são criadas via HTTP real pelo setup() do k6,
-- garantindo que o envelope assinado (HMAC) seja transferido normalmente ao Monitor.

\if :{?total_alunos}
\else
\set total_alunos 100
\endif

BEGIN;

-- 1. Limpa execuções anteriores de estresse sem afetar outros dados de dev
DELETE FROM atribuicoes
WHERE turma_id IN (SELECT id FROM turmas WHERE titulo = 'T-STRESS-100');

DELETE FROM atividades
WHERE professor_id IN (SELECT id FROM users WHERE email = 'prof.stress@seno.dev');

DELETE FROM tarefas
WHERE professor_id IN (SELECT id FROM users WHERE email = 'prof.stress@seno.dev');

DELETE FROM matriculas
WHERE turma_id IN (SELECT id FROM turmas WHERE titulo = 'T-STRESS-100');

DELETE FROM turmas WHERE titulo = 'T-STRESS-100';
DELETE FROM materias WHERE codigo = 'STRESS100';
DELETE FROM periodos_letivos WHERE trimestre = 4 AND ano = 2029;

-- 2. Professor do teste de estresse (reaproveita o hash Argon2/Bcrypt do super@seno.dev)
WITH prof_person AS (
    INSERT INTO persons (first_name, last_name)
    SELECT 'Professor', 'Estresse'
    WHERE NOT EXISTS (
        SELECT 1 FROM users WHERE email = 'prof.stress@seno.dev'
    )
    RETURNING id
)
INSERT INTO users (person_id, email, password_hash, status)
SELECT pp.id,
       'prof.stress@seno.dev',
       (SELECT password_hash FROM users WHERE email = 'super@seno.dev'),
       'active'
FROM prof_person pp;

UPDATE users
SET status = 'active',
    password_hash = (SELECT password_hash FROM users WHERE email = 'super@seno.dev')
WHERE email = 'prof.stress@seno.dev';

INSERT INTO user_roles (user_id, role)
SELECT id, 'professor'
FROM users
WHERE email = 'prof.stress@seno.dev'
ON CONFLICT DO NOTHING;

-- 3. Alunos numerados: stress.aluno001@seno.dev .. stress.alunoNNN@seno.dev
SELECT set_config('seno.stress_alunos', (:'total_alunos')::text, true);

DO $$
DECLARE
    v_total int := COALESCE(current_setting('seno.stress_alunos', true)::int, 100);
    v_hash  text;
    v_email text;
    v_pid   bigint;
    v_uid   bigint;
    i       int;
BEGIN
    SELECT password_hash INTO v_hash FROM users WHERE email = 'super@seno.dev';
    IF v_hash IS NULL THEN
        RAISE EXCEPTION 'super@seno.dev não encontrado no banco. Suba a API uma vez antes do seed.';
    END IF;

    FOR i IN 1..v_total LOOP
        v_email := format('stress.aluno%s@seno.dev', lpad(i::text, 3, '0'));
        SELECT id INTO v_uid FROM users WHERE email = v_email;
        IF v_uid IS NULL THEN
            INSERT INTO persons (first_name, last_name)
            VALUES ('Aluno', format('Estresse %s', lpad(i::text, 3, '0')))
            RETURNING id INTO v_pid;

            INSERT INTO users (person_id, email, password_hash, status)
            VALUES (v_pid, v_email, v_hash, 'active')
            RETURNING id INTO v_uid;
        ELSE
            UPDATE users SET status = 'active', password_hash = v_hash WHERE id = v_uid;
        END IF;

        INSERT INTO user_roles (user_id, role)
        VALUES (v_uid, 'student')
        ON CONFLICT DO NOTHING;
    END LOOP;
END;
$$;

-- 4. Matéria, período, turma e matrículas ativas dos N alunos
INSERT INTO materias (codigo, nome)
VALUES ('STRESS100', 'Laboratório de Estresse — Simulação de Prova');

INSERT INTO periodos_letivos (trimestre, ano)
VALUES (4, 2029);

INSERT INTO turmas (materia_id, periodo_id, professor_id, titulo)
SELECT m.id, p.id, u.id, 'T-STRESS-100'
FROM materias m, periodos_letivos p, users u
WHERE m.codigo = 'STRESS100'
  AND p.trimestre = 4 AND p.ano = 2029
  AND u.email = 'prof.stress@seno.dev';

INSERT INTO matriculas (turma_id, aluno_id, created_by)
SELECT t.id, u.id, t.professor_id
FROM turmas t
JOIN users u ON u.email LIKE 'stress.aluno%@seno.dev'
WHERE t.titulo = 'T-STRESS-100'
  AND substring(u.email FROM 'stress\.aluno([0-9]+)@seno\.dev')::int <= (:'total_alunos')::int;

COMMIT;
