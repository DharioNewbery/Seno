# API routes and permissions

Source: `api/internal/httpapi/routes.go` and `monitor/internal/httpapi/routes.go`.

## Global rules

All routes go through `RequestLog` + `OriginGuard`. `OriginGuard` applies to **POST/PUT/PATCH/DELETE**: the body must be `Content-Type: application/json` and, if an `Origin` header is present, it must match the configured web origin. GETs are unaffected.

- **Public**: no session required.
- **Auth**: `Authorization: Bearer <token>`; session + roles are re-read from the DB on every request (`RequireAuth`), 401 `SESSAO_INVALIDA` if missing/invalid.
- **Staff** = `admin` or `super`.
- **Owner** (`dono`) = the actor owns the resource (turma/professor/tarefa). Out-of-scope access usually returns **404** to hide existence.
- Roles are cumulative; "professor+staff" means any of `professor`, `admin`, `super`.

---

## API service (`api/internal/httpapi/routes.go`)

### Health

| Method | Route | What it does | Permissions |
|---|---|---|---|
| GET | `/healthz` | Process + DB health check | Public |
| GET | `/v1/healthz` | Same, versioned | Public |

### Authentication

| Method | Route | What it does | Permissions |
|---|---|---|---|
| POST | `/v1/auth/login` | Email/password login; returns opaque token, user and roles | Public |
| POST | `/v1/auth/invite/accept` | Consumes one-time invite token and sets the initial password | Public |
| POST | `/v1/auth/password/reset-request` | Sends password-reset email; always returns the same response (anti-enumeration) | Public |
| POST | `/v1/auth/password/reset` | Consumes one-time reset token, sets new password, revokes all sessions | Public |
| POST | `/v1/auth/logout` | Ends the current session | Any authenticated user |
| GET | `/v1/auth/me` | Current user with roles revalidated from DB | Any authenticated user |

### User management (admin portal)

Group requires **admin or super** via middleware. Services add the restrictions below.

| Method | Route | What it does | Permissions |
|---|---|---|---|
| GET | `/v1/users` | List/search users (`busca`, `status`, `cargo`, pagination) | Admin/super. Super users are hidden unless the caller is the super |
| POST | `/v1/users` | Creates a pending user **without roles**, sends invite email | Admin/super |
| GET | `/v1/users/:id` | Detailed user inspection | Admin/super. Super is visible only to itself |
| PUT | `/v1/users/:id/cargos` | Replaces the user's roles | Admin/super. Admin may assign only `student`/`professor`; only super may assign `admin`; `super` is never assignable. Admin/super targets only editable by super (self-edit allowed while keeping admin/super); cannot remove own admin/super |
| PATCH | `/v1/users/:id` | Updates first/last name | Admin/super. Admin accounts only editable by super, except editing self |
| POST | `/v1/users/:id/desativar` | Disables account; active sessions die | Admin/super. Cannot disable super or self; admin targets need super |
| POST | `/v1/users/:id/ativar` | Re-enables a disabled account | Admin/super. Admin targets need super |
| POST | `/v1/users/:id/resetar-senha` | Sends password-reset email | Admin/super. Admin targets need super |

### Ensino — matérias, períodos, turmas, matrículas

**Matérias**

| Method | Route | What it does | Permissions |
|---|---|---|---|
| GET | `/v1/materias` | List subjects | Professor, admin, super |
| POST | `/v1/materias` | Create subject | Admin, super |
| PATCH | `/v1/materias/:id` | Edit subject | Admin, super |
| DELETE | `/v1/materias/:id` | Soft-delete subject | Admin, super |

**Períodos**

| Method | Route | What it does | Permissions |
|---|---|---|---|
| GET | `/v1/periodos` | List academic periods | Professor, admin, super |
| POST | `/v1/periodos` | Create period | Admin, super |
| DELETE | `/v1/periodos/:id` | Delete period; 409 if referenced | Admin, super |

**Turmas**

| Method | Route | What it does | Permissions |
|---|---|---|---|
| GET | `/v1/turmas` | List classes; filters by professor/subject/period/closed | Professor sees only own (forced filter); staff sees all |
| POST | `/v1/turmas` | Create class; actor becomes owner | Role `professor` required (a pure admin cannot create); admin/super also pass the route but need the professor role |
| GET | `/v1/turmas/:id` | Class detail | Owner or staff; 404 outside scope |
| PATCH | `/v1/turmas/:id` | Edit class title | Owner or staff; 409 if class already closed |
| POST | `/v1/turmas/:id/encerrar` | Close class | Owner or staff; 409 if already closed |
| DELETE | `/v1/turmas/:id` | Delete class | Owner or staff; 409 if active enrollments |
| GET | `/v1/turmas/:id/matriculas` | List enrollments (`incluir_encerradas`) | Owner or staff |
| POST | `/v1/turmas/:id/matriculas` | Enroll student by email; unknown student becomes pending + invite | Owner or staff |
| GET | `/v1/turmas/:id/analytics` | Class analytics | Owner or staff |
| GET | `/v1/turmas/:id/alunos/:aluno_id/analytics` | One student's analytics in the class | Owner or staff |

**Matrículas / busca de alunos**

| Method | Route | What it does | Permissions |
|---|---|---|---|
| DELETE | `/v1/matriculas/:id` | Ends enrollment ("remove student") | Class owner or staff |
| GET | `/v1/alunos` | Search students (`busca`, `status`, pagination), fixed `student` role | Professor, admin, super |

### Tarefas (task bank)

| Method | Route | What it does | Permissions |
|---|---|---|---|
| GET | `/v1/tarefas` | List task-bank tasks | Professor sees only own; staff sees all |
| POST | `/v1/tarefas` | Create task with optional tests; actor = owner | Role `professor` required; pure admin cannot create |
| GET | `/v1/tarefas/:id` | Task detail + tests | Owner or staff; 404 otherwise |
| PATCH | `/v1/tarefas/:id` | Edit task; non-nil tests replace the whole list | Owner or staff |
| POST | `/v1/tarefas/:id/duplicar` | Duplicate task | Owner or staff |
| DELETE | `/v1/tarefas/:id` | Delete task | Owner or staff |
| GET | `/v1/tarefas/:id/analytics` | Task usage analytics | Owner or staff |

### Atividades (activity bank)

| Method | Route | What it does | Permissions |
|---|---|---|---|
| GET | `/v1/atividades` | List activities | Professor sees only own; staff sees all |
| POST | `/v1/atividades` | Create activity; actor = owner | Role `professor` required; pure admin cannot create |
| GET | `/v1/atividades/:id` | Activity detail | Owner or staff; 404 otherwise |
| PATCH | `/v1/atividades/:id` | Edit activity; tasks replace list | Owner or staff |
| POST | `/v1/atividades/:id/duplicar` | Duplicate activity | Owner or staff |
| DELETE | `/v1/atividades/:id` | Delete activity | Owner or staff |

### Atribuições (class ← activity)

| Method | Route | What it does | Permissions |
|---|---|---|---|
| GET | `/v1/atribuicoes` | List assignments; filters by class/activity | Staff sees all; professor sees only own classes' assignments |
| POST | `/v1/atribuicoes` | Assign activity to class; also queues Monitor transfer | Class owner or staff; activity must belong to the class's professor |
| GET | `/v1/atribuicoes/:id` | Assignment detail | Class owner or staff; 404 otherwise |
| PATCH | `/v1/atribuicoes/:id` | Edit timing/flags (nil preserves) | Owner or staff |
| DELETE | `/v1/atribuicoes/:id` | Unlink activity from class | Owner or staff |
| GET | `/v1/atribuicoes/:id/analytics` | Assignment analytics | Owner or staff |

### Tentativa (student draft)

Route only requires authentication; the service enforces **role `student` + active enrollment**. Professors/admins get 403; no enrollment → 404.

| Method | Route | What it does | Permissions |
|---|---|---|---|
| POST | `/v1/atribuicoes/:id/tentativa` | Opens/returns the student's draft (idempotent, sets `comecou_em`) | Student + active enrollment |
| GET | `/v1/atribuicoes/:id/tentativa` | Reads own draft (404 if not opened) | Student + active enrollment |
| PUT | `/v1/atribuicoes/:id/tentativa` | Autosave with optimistic revision; stale revision → 409 | Student + active enrollment |

### Submissões / Monitor actions under an assignment

Route only requires authentication; services/handlers enforce the rules.

| Method | Route | What it does | Permissions |
|---|---|---|---|
| POST | `/v1/atribuicoes/:id/submissao` | Student submits current draft snapshot; creates submission + automatic correction; duplicate submission → 409 | Student + active enrollment |
| GET | `/v1/atribuicoes/:id/submissoes` | List submissions for assignment (includes Monitor mirror) | Class owner or staff |
| POST | `/v1/atribuicoes/:id/coletar/:aluno_id` | Collects an expired draft when late delivery is not allowed; falls back to Monitor | Class owner or staff |
| POST | `/v1/atribuicoes/:id/correcoes/publicar` | Batch-publishes confirmed, unpublished corrections | Class owner or staff |
| POST | `/v1/atribuicoes/:id/monitor/alunos/:aluno_id/reexecutar` | Re-runs a student's correction in the Monitor | Class owner or staff; assignment must have a `monitor_id` |
| DELETE | `/v1/atribuicoes/:id/monitor/alunos/:aluno_id/submissao` | Deletes a student's submission in the Monitor | Class owner or staff; assignment must have a `monitor_id` |
| GET | `/v1/atribuicoes/:id/monitor/alunos/:aluno_id/submissao` | Fetches a student's submission from the Monitor | Class owner or staff; assignment must have a `monitor_id` |
| GET | `/v1/atribuicoes/:id/entrar` | Issues short-lived SSO token + Monitor entry URL | Student + active enrollment; assignment transferred to Monitor |

### Correções (submission corrections)

Route only requires authentication; services enforce ownership.

| Method | Route | What it does | Permissions |
|---|---|---|---|
| DELETE | `/v1/submissoes/:id` | Deletes submission + correction; restores a prefilled draft for the student | Class owner or staff |
| GET | `/v1/submissoes/:id/correcao` | Full correction view | Class owner or staff |
| PATCH | `/v1/submissoes/:id/correcao` | Edit general feedback and per-task grades/feedback | Class owner or staff; 409 if already published |
| POST | `/v1/submissoes/:id/correcao/confirmar` | Confirms correction (records who confirmed) | Class owner or staff |
| POST | `/v1/submissoes/:id/correcao/publicar` | Publishes correction to the student; requires confirmation | Class owner or staff |
| POST | `/v1/submissoes/:id/correcao/reexecutar` | Re-runs automatic grading; only `falhou` or unconfirmed `concluida` | Class owner or staff |
| GET | `/v1/submissoes/:id/minha` | Student's view of own correction; before publishing only status | Student owner of the submission |

### Judge0 callback

| Method | Route | What it does | Permissions |
|---|---|---|---|
| POST | `/v1/judge0/callback?key=&par=ct:te` | Receives Judge0 verdicts and applies them to corrections | No session; must present the shared `SENO_JUDGE0_CALLBACK_KEY`; also subject to `OriginGuard` (JSON content type) |

### Test execution / student portal / metrics

| Method | Route | What it does | Permissions |
|---|---|---|---|
| POST | `/v1/atribuicoes/:id/testar` | Student runs current code against **public** tests; 7-day history, 5 runs/min | Student + active enrollment + open attempt/window |
| GET | `/v1/me/testes` | Student's test runs from the last 7 days; optional `atribuicao_id` | Student |
| POST | `/v1/tarefas/:id/testar` | Professor tests a task-bank task with any language against all tests; no history | Task owner or staff |
| GET | `/v1/atribuicoes/:id/aluno` | Student-facing assignment material (activity/tasks/draft), private data filtered server-side | Student + active enrollment |
| GET | `/v1/me/turmas` | Student's classes with active enrollment | Student |
| GET | `/v1/me/turmas/:id` | Student's class detail (teacher, colleagues, assignments/status) | Student with active enrollment in that class |
| GET | `/v1/me/atribuicoes` | Student's assignments across active enrollments; optional `turma_id` | Student |
| GET | `/v1/admin/metricas` | Global platform metrics | Admin or super only |

---

## Monitor service (`monitor/internal/httpapi/routes.go`) — separate service

| Method | Route | What it does | Permissions |
|---|---|---|---|
| GET | `/healthz` | Process + DB health | Public |
| POST | `/interna/v1/atividades` | Receives a full activity transfer from the API (idempotent by `api_id`) | Internal HMAC signature + timestamp window |
| GET | `/entrar?st=...` | Validates SSO entry token and issues `hm_sess` cookie; redirects browser to exam UI | Public endpoint, but requires valid signed short-lived `st`; 10 attempts/5 min per IP |
| GET | `/interna/v1/atividades/:monitor_id/estado` | Returns consolidated state/snapshot for an activity | Internal HMAC |
| POST | `/interna/v1/atividades/:monitor_id/alunos/:aluno_id/reexecutar` | Re-runs correction for a student in Monitor | Internal HMAC |
| DELETE | `/interna/v1/atividades/:monitor_id/alunos/:aluno_id/submissao` | Deletes a student's submission in Monitor | Internal HMAC |
| GET | `/interna/v1/atividades/:monitor_id/alunos/:aluno_id/submissao` | Fetches a student's submission from Monitor | Internal HMAC |
| POST | `/interna/v1/atividades/:monitor_id/alunos/:aluno_id/coletar` | Collects a student's draft in Monitor | Internal HMAC |
| POST | `/a/atividade/:monitor_id/abrir` | Opens/returns the exam activity (idempotent) | Valid `hm_sess` cookie bound to student + Monitor activity; finalization/window checks |
| PUT | `/a/atividade/:monitor_id/tentativa` | Autosave draft with optimistic revision | Same student session |
| POST | `/a/atividade/:monitor_id/testar` | Queues a Judge0 test run (202 + `run_id`) | Same student session; rate limits |
| GET | `/a/atividade/:monitor_id/testar/:run_id` | Polls a test run result (202/200) | Same student session |
| POST | `/a/atividade/:monitor_id/submissao/avaliar` | Synchronous pre-evaluation; returns report + `pre_id`/expiry | Same student session |
| POST | `/a/atividade/:monitor_id/submissao/cancelar` | Discards pre-evaluation, keeps draft alive | Same student session |
| POST | `/a/atividade/:monitor_id/submissao/confirmar` | Confirms final submission with choice | Same student session |
| GET | `/a/app`, `/a/app/` | Serves exam SPA | Public static asset |
| GET | `/a/app/app.css`, `/a/app.css` | Exam CSS | Public static asset |
| GET | `/a/app/app.js`, `/a/app.js` | Exam JS | Public static asset |
| PUT | `/interna/callbacks/judge0?key=&run=&t=` | Receives Judge0 verdicts for Monitor test runs | Shared Judge0 callback key (`SENO_JUDGE0_CALLBACK_SECRET`) |

---

Not checked: frontend/web calls (`web/`) and whether any external proxy adds extra auth. Risk: middleware-level roles are clear, but several routes rely on **service-level** ownership checks; if those services change, the "owner or staff" entries can drift.
