-- Queries de ensino: matérias, períodos letivos, turmas e matrículas
-- (PROJETO §Organização de Matérias/Turmas, §Matrícula).

-- ============================ Matérias ============================

-- name: InsertMateria :one
INSERT INTO materias (codigo, nome, created_by)
VALUES ($1, $2, $3)
RETURNING id, codigo, nome, created_at;

-- name: UpdateMateria :one
UPDATE materias
SET codigo = $2,
    nome   = $3
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, codigo, nome, created_at;

-- name: SoftDeleteMateria :execrows
UPDATE materias
SET deleted_at = now()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: GetMateria :one
SELECT id, codigo, nome, created_at
FROM materias
WHERE id = $1
  AND deleted_at IS NULL;

-- name: ListMaterias :many
SELECT id, codigo, nome, created_at
FROM materias
WHERE deleted_at IS NULL
ORDER BY codigo;

-- ========================== Períodos =============================

-- name: FindOrCreatePeriodo :one
INSERT INTO periodos_letivos (trimestre, ano)
VALUES ($1, $2)
ON CONFLICT (trimestre, ano) DO UPDATE
SET trimestre = periodos_letivos.trimestre
RETURNING id, trimestre, ano;

-- name: ListPeriodos :many
SELECT id, trimestre, ano, created_at
FROM periodos_letivos
ORDER BY ano DESC, trimestre DESC;

-- ============================ Turmas =============================

-- name: InsertTurma :one
INSERT INTO turmas (materia_id, periodo_id, professor_id, titulo, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at;

-- name: GetTurma :one
SELECT t.id,
       t.materia_id,
       t.periodo_id,
       t.professor_id,
       t.titulo,
       t.encerrado_em,
       t.created_at,
       m.codigo    AS materia_codigo,
       m.nome      AS materia_nome,
       p.trimestre AS periodo_trimestre,
       p.ano       AS periodo_ano,
       up.first_name AS prof_first_name,
       up.last_name  AS prof_last_name,
       uu.email       AS prof_email
FROM turmas t
JOIN materias m ON m.id = t.materia_id
JOIN periodos_letivos p ON p.id = t.periodo_id
JOIN users uu ON uu.id = t.professor_id
JOIN persons up ON up.id = uu.person_id
WHERE t.id = $1;

-- name: ListTurmas :many
SELECT t.id,
       t.materia_id,
       t.periodo_id,
       t.professor_id,
       t.titulo,
       t.encerrado_em,
       t.created_at,
       m.codigo    AS materia_codigo,
       m.nome      AS materia_nome,
       p.trimestre AS periodo_trimestre,
       p.ano       AS periodo_ano,
       up.first_name AS prof_first_name,
       up.last_name  AS prof_last_name,
       uu.email       AS prof_email
FROM turmas t
JOIN materias m ON m.id = t.materia_id
JOIN periodos_letivos p ON p.id = t.periodo_id
JOIN users uu ON uu.id = t.professor_id
JOIN persons up ON up.id = uu.person_id
WHERE (sqlc.narg('professor')::bigint IS NULL OR t.professor_id = sqlc.narg('professor')::bigint)
  AND (sqlc.narg('materia')::bigint IS NULL OR t.materia_id = sqlc.narg('materia')::bigint)
  AND (sqlc.narg('periodo')::bigint IS NULL OR t.periodo_id = sqlc.narg('periodo')::bigint)
  AND (
    (sqlc.narg('encerradas')::text IS NULL AND t.encerrado_em IS NULL)
    OR (sqlc.narg('encerradas')::text = 'only' AND t.encerrado_em IS NOT NULL)
    OR (sqlc.narg('encerradas')::text = 'all')
  )
ORDER BY t.created_at DESC, t.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountTurmas :one
SELECT count(*)
FROM turmas t
WHERE (sqlc.narg('professor')::bigint IS NULL OR t.professor_id = sqlc.narg('professor')::bigint)
  AND (sqlc.narg('materia')::bigint IS NULL OR t.materia_id = sqlc.narg('materia')::bigint)
  AND (sqlc.narg('periodo')::bigint IS NULL OR t.periodo_id = sqlc.narg('periodo')::bigint)
  AND (
    (sqlc.narg('encerradas')::text IS NULL AND t.encerrado_em IS NULL)
    OR (sqlc.narg('encerradas')::text = 'only' AND t.encerrado_em IS NOT NULL)
    OR (sqlc.narg('encerradas')::text = 'all')
  );

-- name: UpdateTurma :exec
UPDATE turmas
SET titulo = $2
WHERE id = $1;

-- name: EncerrarTurma :execrows
UPDATE turmas
SET encerrado_em = now()
WHERE id = $1
  AND encerrado_em IS NULL;

-- name: CountMatriculasAtivasTurma :one
SELECT count(*)
FROM matriculas
WHERE turma_id = $1
  AND saida_em IS NULL;

-- name: DeleteTurma :execrows
DELETE FROM turmas
WHERE id = $1;

-- ========================== Matrículas ===========================

-- name: InsertMatricula :one
INSERT INTO matriculas (turma_id, aluno_id, created_by)
VALUES ($1, $2, $3)
RETURNING id, entrada_em, created_at;

-- name: EncerrarMatricula :execrows
UPDATE matriculas
SET saida_em = now()
WHERE id = $1
  AND saida_em IS NULL;

-- name: GetMatricula :one
SELECT mt.id,
       mt.turma_id,
       mt.aluno_id,
       mt.entrada_em,
       mt.saida_em,
       mt.created_at,
       t.professor_id AS turma_professor_id,
       t.encerrado_em AS turma_encerrado_em
FROM matriculas mt
JOIN turmas t ON t.id = mt.turma_id
WHERE mt.id = $1;

-- name: ListMatriculas :many
SELECT mt.id,
       mt.aluno_id,
       mt.entrada_em,
       mt.saida_em,
       u.email        AS aluno_email,
       u.status       AS aluno_status,
       p.first_name   AS aluno_first_name,
       p.last_name    AS aluno_last_name
FROM matriculas mt
JOIN users u ON u.id = mt.aluno_id
JOIN persons p ON p.id = u.person_id
WHERE mt.turma_id = $1
  AND (mt.saida_em IS NULL OR $2::boolean)
ORDER BY (mt.saida_em IS NULL) DESC, mt.entrada_em, mt.id;
