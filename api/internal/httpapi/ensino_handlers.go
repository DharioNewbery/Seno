// Handlers de ensino: matérias, períodos, turmas e matrículas. O escopo
// (§5.4) é aplicado no serviço: professor só as próprias turmas (404);
// admin/super tudo.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/seno-project/seno/api/internal/domain"
	"github.com/seno-project/seno/api/internal/ensino"
	"github.com/seno-project/seno/api/internal/platform"
	"github.com/seno-project/seno/api/internal/usuarios"
)

// ------------------------------ Matérias ------------------------------

// ListarMaterias: GET /v1/materias (professor+staff; leitura).
func (d *Dependencies) ListarMaterias(c *gin.Context) {
	materias, err := d.Ensino.ListarMaterias(c.Request.Context())
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"materias": materias})
}

// CriarMateria: POST /v1/materias (staff).
func (d *Dependencies) CriarMateria(c *gin.Context) {
	user, _ := currentUser(c)
	var cad ensino.CadastroMateria
	if !bindJSON(c, &cad) {
		return
	}
	materia, err := d.Ensino.CriarMateria(c.Request.Context(), user, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusCreated, materia)
}

// EditarMateria: PATCH /v1/materias/:id (staff).
func (d *Dependencies) EditarMateria(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var cad ensino.CadastroMateria
	if !bindJSON(c, &cad) {
		return
	}
	materia, err := d.Ensino.EditarMateria(c.Request.Context(), user, id, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, materia)
}

// ExcluirMateria: DELETE /v1/materias/:id (staff; soft delete).
func (d *Dependencies) ExcluirMateria(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	if err := d.Ensino.ExcluirMateria(c.Request.Context(), user, id); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ----------------------------- Períodos ------------------------------

// ListarPeriodos: GET /v1/periodos (professor+staff).
func (d *Dependencies) ListarPeriodos(c *gin.Context) {
	periodos, err := d.Ensino.ListarPeriodos(c.Request.Context())
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"periodos": periodos})
}

// CriarPeriodo: POST /v1/periodos (staff).
func (d *Dependencies) CriarPeriodo(c *gin.Context) {
	user, _ := currentUser(c)
	var cad ensino.CadastroPeriodo
	if !bindJSON(c, &cad) {
		return
	}
	periodo, err := d.Ensino.CriarPeriodo(c.Request.Context(), user, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusCreated, periodo)
}

// ListarAlunos: GET /v1/alunos?busca&status&pagina&por — busca de alunos
// para matricular (professor+staff; cargo de student fixo no filtro),
// devolvendo {total, usuarios}.
func (d *Dependencies) ListarAlunos(c *gin.Context) {
	pagina, por := numPagina(c)
	lista, err := d.UsuarioS.Listar(c.Request.Context(), usuarios.Filtros{
		Busca:  c.Query("busca"),
		Status: c.Query("status"),
		Cargo:  domain.RoleStudent,
		Pagina: pagina,
		Por:    por,
	})
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"total":    lista.Total,
		"pagina":   lista.Pagina,
		"por":      lista.Por,
		"usuarios": lista.Usuarios,
	})
}

// ------------------------------- Turmas ------------------------------

// numParamOpcional lê ?professor_id/&materia_id como filtro (staff).
func numParamOpcional(c *gin.Context, nome string) (*int64, error) {
	valor := c.Query(nome)
	if valor == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(valor, 10, 64)
	if err != nil || n < 1 {
		return nil, platform.NewAPIError(
			http.StatusUnprocessableEntity, platform.CodeUnprocessable,
			"Filtro "+nome+" inválido.")
	}
	return &n, nil
}

// ListarTurmas: GET /v1/turmas — professor recebe apenas as próprias
// (filtro forçado no serviço); admin/super podem filtrar por professor.
func (d *Dependencies) ListarTurmas(c *gin.Context) {
	user, _ := currentUser(c)
	prof, err := numParamOpcional(c, "professor_id")
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	mat, err := numParamOpcional(c, "materia_id")
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	per, err := numParamOpcional(c, "periodo_id")
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	pagina, por := numPagina(c)
	lista, err := d.Ensino.ListarTurmas(c.Request.Context(), user, ensino.FiltroTurmas{
		ProfessorID: prof,
		MateriaID:   mat,
		PeriodoID:   per,
		Encerradas:  c.Query("encerradas"),
		Pagina:      &pagina,
		Por:         &por,
	})
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, lista)
}

// CriaTurma: POST /v1/turmas — cargo professor obrigatório; dono = ator.
func (d *Dependencies) CriaTurma(c *gin.Context) {
	user, _ := currentUser(c)
	var cad ensino.CadastroTurma
	if !bindJSON(c, &cad) {
		return
	}
	turma, err := d.Ensino.CriaTurma(c.Request.Context(), user, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusCreated, turma)
}

// VerTurma: GET /v1/turmas/:id — 404 fora do escopo do professor.
func (d *Dependencies) VerTurma(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	turma, err := d.Ensino.VerTurma(c.Request.Context(), user, id)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, turma)
}

// EditarTurma: PATCH /v1/turmas/:id — título; dono|staff.
func (d *Dependencies) EditarTurma(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var cad ensino.EditarTurma
	if !bindJSON(c, &cad) {
		return
	}
	turma, err := d.Ensino.EditarTurma(c.Request.Context(), user, id, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, turma)
}

// EncerrarTurma: POST /v1/turmas/:id/encerrar — dono|staff.
func (d *Dependencies) EncerrarTurma(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	if err := d.Ensino.EncerrarTurma(c.Request.Context(), user, id); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ExcluirTurma: DELETE /v1/turmas/:id — dono|staff; sem matrículas ativas.
func (d *Dependencies) ExcluirTurma(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	if err := d.Ensino.ExcluirTurma(c.Request.Context(), user, id); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ----------------------------- Matrículas ----------------------------

// ListarMatriculasTurma: GET /v1/turmas/:id/matriculas
// ?incluir_encerradas=true (padrão: só ativas).
func (d *Dependencies) ListarMatriculasTurma(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	incluir := c.Query("incluir_encerradas") == "true"
	lista, err := d.Ensino.ListarMatriculas(c.Request.Context(), user, id, incluir)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusOK, lista)
}

// MatricularTurma: POST /v1/turmas/:id/matriculas — dono|staff;
// aluno inexistente nasce pendente com convite (ARQUITETURA §5.3).
func (d *Dependencies) MatricularTurma(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	var cad ensino.CadastroMatricula
	if !bindJSON(c, &cad) {
		return
	}
	mat, err := d.Ensino.Matricular(c.Request.Context(), user, id, cad)
	if err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.JSON(http.StatusCreated, mat)
}

// EncerrarMatricula: DELETE /v1/matriculas/:id — dono da turma|staff.
func (d *Dependencies) EncerrarMatricula(c *gin.Context) {
	user, _ := currentUser(c)
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	if err := d.Ensino.EncerrarMatricula(c.Request.Context(), user, id); err != nil {
		platform.ErrorBody(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// numPagina lê ?pagina/?por com limites padrão (para turmas).
func numPagina(c *gin.Context) (int, int) {
	pagina, por := 1, 50
	if n, err := strconv.Atoi(c.Query("pagina")); err == nil && n > 0 {
		pagina = n
	}
	if n, err := strconv.Atoi(c.Query("por")); err == nil && n > 0 {
		por = n
	}
	if por > 200 {
		por = 200
	}
	return pagina, por
}
