-- Queries de usuários, pessoas e cargos (CRUD admin + carregamentos de sessão).

-- name: GetUserByEmail :one
SELECT u.id,
       u.person_id,
       u.email,
       u.password_hash,
       u.status,
       u.disabled_at,
       u.created_by,
       u.created_at,
       u.updated_at,
       p.first_name,
       p.last_name
FROM users u
JOIN persons p ON p.id = u.person_id
WHERE lower(u.email) = lower($1);

-- name: GetUserByID :one
SELECT u.id,
       u.person_id,
       u.email,
       u.password_hash,
       u.status,
       u.disabled_at,
       u.created_by,
       u.created_at,
       u.updated_at,
       p.first_name,
       p.last_name
FROM users u
JOIN persons p ON p.id = u.person_id
WHERE u.id = $1;

-- name: GetUserRoles :many
SELECT role
FROM user_roles
WHERE user_id = $1
ORDER BY role;

-- name: InsertPerson :one
INSERT INTO persons (first_name, last_name)
VALUES ($1, $2)
RETURNING id;

-- name: UpdatePerson :exec
UPDATE persons
SET first_name = $1,
    last_name  = $2
WHERE id = $3;

-- name: InsertUser :one
INSERT INTO users (person_id, email, password_hash, status, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at;

-- name: UpdateUserEmail :exec
UPDATE users
SET email = $1
WHERE id = $2;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2
WHERE id = $1;

-- name: ActivateUser :exec
UPDATE users
SET status = 'active'
WHERE id = $1
  AND status = 'pending';

-- name: DisableUser :exec
UPDATE users
SET status      = 'disabled',
    disabled_at = now(),
    updated_by  = $2
WHERE id = $1;

-- name: EnableUser :exec
UPDATE users
SET status      = 'active',
    disabled_at = NULL,
    updated_by  = $2
WHERE id = $1;

-- name: SetUserUpdatedBy :exec
UPDATE users
SET updated_by = $2
WHERE id = $1;

-- name: DeleteUserViaPerson :exec
DELETE FROM persons
WHERE id = (SELECT person_id FROM users WHERE users.id = $1);

-- name: CountUsers :one
SELECT count(*)
FROM users u
JOIN persons p ON p.id = u.person_id
WHERE (COALESCE($1::text, '') = ''
    OR u.email ILIKE '%' || $1 || '%'
    OR p.first_name ILIKE '%' || $1 || '%'
    OR p.last_name ILIKE '%' || $1 || '%')
  AND (COALESCE($2::text, '') = '' OR u.status = $2::text)
  AND (COALESCE($3::text, '') = '' OR EXISTS (
        SELECT 1 FROM user_roles r WHERE r.user_id = u.id AND r.role = $3::text))
  AND (NOT $4::boolean OR NOT EXISTS (
        SELECT 1 FROM user_roles r
        WHERE r.user_id = u.id AND r.role = 'super'));

-- name: ListUsers :many
SELECT u.id,
       u.email,
       u.status,
       u.disabled_at,
       u.created_at,
       u.updated_at,
       p.first_name,
       p.last_name
FROM users u
JOIN persons p ON p.id = u.person_id
WHERE (COALESCE($1::text, '') = ''
    OR u.email ILIKE '%' || $1 || '%'
    OR p.first_name ILIKE '%' || $1 || '%'
    OR p.last_name ILIKE '%' || $1 || '%')
  AND (COALESCE($2::text, '') = '' OR u.status = $2::text)
  AND (COALESCE($3::text, '') = '' OR EXISTS (
        SELECT 1 FROM user_roles r
        WHERE r.user_id = u.id AND r.role = $3::text))
  AND (NOT $4::boolean OR NOT EXISTS (
        SELECT 1 FROM user_roles r
        WHERE r.user_id = u.id AND r.role = 'super'))
ORDER BY u.created_at DESC, u.id DESC
LIMIT $5
OFFSET $6;

-- name: ListUserRolesByIDs :many
SELECT user_id, role
FROM user_roles
WHERE user_id = ANY($1::bigint[])
ORDER BY user_id, role;

-- name: DeleteUserRoles :exec
DELETE FROM user_roles
WHERE user_id = $1;

-- name: InsertUserRole :exec
INSERT INTO user_roles (user_id, role, created_at)
VALUES ($1, $2, now());
