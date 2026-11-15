package contract

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizarTipoTarefa(t *testing.T) {
	if got := NormalizarTipoTarefa(""); got != TipoTarefaProgramacao {
		t.Fatalf("esperado %q, obtido %q", TipoTarefaProgramacao, got)
	}
	if got := NormalizarTipoTarefa("  ARQUIVO_SUBMISSAO "); got != TipoTarefaArquivo {
		t.Fatalf("esperado %q, obtido %q", TipoTarefaArquivo, got)
	}
	if !TipoTarefaValido(TipoTarefaProgramacao) || !TipoTarefaValido(TipoTarefaArquivo) {
		t.Fatal("tipos válidos rejeitados")
	}
	if TipoTarefaValido("desconhecido") {
		t.Fatal("tipo inválido aceito")
	}
}

func TestNormalizarConfigArquivo(t *testing.T) {
	cfg, err := NormalizarConfigArquivo(ConfigTarefaArquivo{
		MaxBytes:            5 * 1024 * 1024,
		ExtensoesPermitidas: []string{"PDF", ".zip, py "},
	}, DefaultArquivoMaxBytes)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if cfg.MaxBytes != 5*1024*1024 {
		t.Fatalf("MaxBytes incorreto: %d", cfg.MaxBytes)
	}
	if len(cfg.ExtensoesPermitidas) != 3 || cfg.ExtensoesPermitidas[0] != ".pdf" || cfg.ExtensoesPermitidas[1] != ".zip" || cfg.ExtensoesPermitidas[2] != ".py" {
		t.Fatalf("extensões incorretas: %+v", cfg.ExtensoesPermitidas)
	}

	// Extensão proibida (.exe)
	if _, err := NormalizarConfigArquivo(ConfigTarefaArquivo{
		ExtensoesPermitidas: []string{".exe"},
	}, DefaultArquivoMaxBytes); err == nil {
		t.Fatal("deveria rejeitar extensão .exe")
	}

	// Tamanho acima do teto global
	if _, err := NormalizarConfigArquivo(ConfigTarefaArquivo{
		MaxBytes: 20 * 1024 * 1024,
	}, 10*1024*1024); err == nil {
		t.Fatal("deveria rejeitar tamanho acima do teto global")
	}
}

func TestSanitizarNomeArquivo(t *testing.T) {
	nome, err := SanitizarNomeArquivo("../../etc/relatorio final;v1.pdf")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if nome != "relatorio final_v1.pdf" {
		t.Fatalf("nome sanitizado inesperado: %q", nome)
	}

	if _, err := SanitizarNomeArquivo(".hidden"); !errors.Is(err, ErrArquivoNomeInvalido) {
		t.Fatalf("deveria rejeitar arquivo oculto: %v", err)
	}
}

func TestValidarArquivo(t *testing.T) {
	cfg := ConfigTarefaArquivo{
		MaxBytes:            1024 * 1024,
		ExtensoesPermitidas: []string{".pdf", ".zip", ".py", ".png"},
	}

	// 1. PDF válido
	pdfBytes := []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n")
	meta, err := ValidarArquivo("entrega_aluno.pdf", pdfBytes, cfg, DefaultArquivoMaxBytes)
	if err != nil {
		t.Fatalf("PDF válido rejeitado: %v", err)
	}
	if meta.MimeType != "application/pdf" || meta.Extensao != ".pdf" || len(meta.Sha256) != 64 {
		t.Fatalf("metadados inesperados: %+v", meta)
	}

	// 2. Python válido
	pyBytes := []byte("def soma(a, b):\n    return a + b\n")
	metaPy, err := ValidarArquivo("solucao.py", pyBytes, cfg, DefaultArquivoMaxBytes)
	if err != nil {
		t.Fatalf("script Python válido rejeitado: %v", err)
	}
	if !strings.HasPrefix(metaPy.MimeType, "text/plain") {
		t.Fatalf("MIME Python inesperado: %q", metaPy.MimeType)
	}

	// 3. Extensão não permitida (.txt quando só .pdf/.zip/.py/.png)
	if _, err := ValidarArquivo("notas.txt", []byte("ola"), cfg, DefaultArquivoMaxBytes); !errors.Is(err, ErrArquivoExtensaoNegada) {
		t.Fatalf("esperava ErrArquivoExtensaoNegada, obteve: %v", err)
	}

	// 4. Executável ELF disfarçado de .pdf
	elfDisfarcado := []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01, 0x01, 0x00}
	if _, err := ValidarArquivo("virus.pdf", elfDisfarcado, cfg, DefaultArquivoMaxBytes); !errors.Is(err, ErrArquivoConteudoNegado) {
		t.Fatalf("esperava ErrArquivoConteudoNegado para ELF disfarçado, obteve: %v", err)
	}

	// 5. Texto renomeado para .pdf (sem magic bytes %PDF-)
	if _, err := ValidarArquivo("falso.pdf", []byte("isto nao e um pdf"), cfg, DefaultArquivoMaxBytes); !errors.Is(err, ErrArquivoConteudoNegado) {
		t.Fatalf("esperava ErrArquivoConteudoNegado para falso PDF, obteve: %v", err)
	}

	// 6. Binário com bytes nulos disfarçado de .py
	if _, err := ValidarArquivo("binario.py", []byte{'p', 'r', 0x00, 0xff}, cfg, DefaultArquivoMaxBytes); !errors.Is(err, ErrArquivoConteudoNegado) {
		t.Fatalf("esperava ErrArquivoConteudoNegado para binário em .py, obteve: %v", err)
	}

	// 7. Arquivo excedendo MaxBytes
	cfgPequeno := ConfigTarefaArquivo{MaxBytes: 1024, ExtensoesPermitidas: []string{".py"}}
	grande := make([]byte, 2048)
	for i := range grande {
		grande[i] = 'a'
	}
	if _, err := ValidarArquivo("grande.py", grande, cfgPequeno, DefaultArquivoMaxBytes); !errors.Is(err, ErrArquivoTamanhoExcedido) {
		t.Fatalf("esperava ErrArquivoTamanhoExcedido, obteve: %v", err)
	}
}
