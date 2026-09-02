package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"seno/internal/models"
)

// Status de submissão ao longo da correção automática.
const (
	SubmissionStatusPending = "pending"
	SubmissionStatusPassed  = "passed"
	SubmissionStatusFailed  = "failed"
	SubmissionStatusError   = "error"
)

type GradingService struct {
	assignmentRepo AssignmentRepository
	runner         CodeRunner
}

func NewGradingService(assignmentRepo AssignmentRepository, runner CodeRunner) *GradingService {
	return &GradingService{assignmentRepo: assignmentRepo, runner: runner}
}

// ListPending retorna as submissões aguardando correção.
func (s *GradingService) ListPending(ctx context.Context, limit int) ([]models.Submission, error) {
	return s.assignmentRepo.ListPendingSubmissions(ctx, limit)
}

// testResult é o resultado de um caso de teste (serializado em JSON).
type testResult struct {
	Position   int    `json:"position"`
	Passed     bool   `json:"passed"`
	Input      string `json:"input"`
	Expected   string `json:"expected"`
	Got        string `json:"got"`
	Stderr     string `json:"stderr,omitempty"`
	TimedOut   bool   `json:"timed_out,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

type gradingResult struct {
	Tests   []testResult `json:"tests"`
	Summary struct {
		Passed int `json:"passed"`
		Total  int `json:"total"`
	} `json:"summary"`
}

// Evaluate corrige uma submissão pendente: roda cada caso de teste em
// container isolado e grava o status final (passed/failed/error) com o
// detalhe por caso. Erros de infraestrutura são devolvidos sem gravar nada:
// a submissão permanece pending e o worker tenta novamente.
func (s *GradingService) Evaluate(ctx context.Context, submission models.Submission) error {
	tests, err := s.assignmentRepo.ListTests(ctx, submission.AssignmentID)
	if err != nil {
		return err
	}

	result := gradingResult{Tests: make([]testResult, 0, len(tests))}
	hasExecError := false
	hasDiff := false

	for _, t := range tests {
		tr := testResult{
			Position: t.Position,
			Input:    t.Input,
			Expected: t.ExpectedOutput,
		}

		run, runErr := s.runner.Run(ctx, RunRequest{
			Language:   submission.Language,
			SourceCode: submission.SourceCode,
			Stdin:      t.Input,
		})
		if runErr != nil {
			return runErr
		}

		tr.Got = run.Stdout
		tr.Stderr = run.Stderr
		tr.TimedOut = run.TimedOut
		tr.ExitCode = run.ExitCode
		tr.DurationMs = run.DurationMs

		switch {
		case run.TimedOut || run.ExitCode != 0:
			hasExecError = true
		case normalizeOutput(run.Stdout) == normalizeOutput(t.ExpectedOutput):
			tr.Passed = true
			result.Summary.Passed++
		default:
			hasDiff = true
		}

		result.Tests = append(result.Tests, tr)
	}
	result.Summary.Total = len(tests)

	status := SubmissionStatusPassed
	switch {
	case hasExecError:
		status = SubmissionStatusError
	case hasDiff:
		status = SubmissionStatusFailed
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("erro ao serializar resultado da correção: %w", err)
	}

	return s.assignmentRepo.UpdateSubmissionResult(ctx, submission.ID, status, string(resultJSON))
}

// normalizeOutput padroniza a saída para comparação: CRLF -> LF, sem
// espaços/tabs finais por linha e sem linhas vazias no início/fim.
func normalizeOutput(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}
