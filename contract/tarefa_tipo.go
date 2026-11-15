package contract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Tipos de tarefa suportados no Seno. Novos tipos devem ser registrados aqui
// e acompanhados de seus validadores/processadores específicos nos serviços.
const (
	TipoTarefaProgramacao = "programacao"
	TipoTarefaArquivo     = "arquivo_submissao"
)

// Limites globais default para tarefas de submissão de arquivo.
const (
	DefaultArquivoMaxBytes int64 = 10 * 1024 * 1024 // 10 MB
	MinArquivoMaxBytes     int64 = 1024             // 1 KB
	MaxArquivoMaxBytes     int64 = 50 * 1024 * 1024 // 50 MB teto absoluto
)

var (
	ErrTipoTarefaInvalido     = errors.New("tipo de tarefa inválido")
	ErrArquivoVazio           = errors.New("o arquivo enviado está vazio")
	ErrArquivoTamanhoExcedido = errors.New("o arquivo excede o tamanho máximo permitido")
	ErrArquivoNomeInvalido    = errors.New("nome de arquivo inválido")
	ErrArquivoExtensaoNegada  = errors.New("extensão de arquivo não permitida para esta tarefa")
	ErrArquivoConteudoNegado  = errors.New("o conteúdo do arquivo não corresponde ao formato permitido ou é um executável bloqueado")
)

// ConfigTarefaArquivo define as restrições configuradas pelo professor para uma
// tarefa do tipo "arquivo_submissao".
type ConfigTarefaArquivo struct {
	MaxBytes            int64    `json:"max_bytes"`
	ExtensoesPermitidas []string `json:"extensoes_permitidas"`
}

// ArquivoSubmetidoMonitor transporta um arquivo submetido pelo aluno do Monitor
// para a API (e na visualização de detalhes).
type ArquivoSubmetidoMonitor struct {
	TarefaOrdem  int    `json:"tarefa_ordem"`
	TarefaAPIID  int64  `json:"tarefa_api_id"`
	NomeArquivo  string `json:"nome_arquivo"`
	MimeType     string `json:"mime_type"`
	TamanhoBytes int64  `json:"tamanho_bytes"`
	Sha256       string `json:"sha256"`
	Conteudo     []byte `json:"conteudo,omitempty"`
	EnviadoEm    string `json:"enviado_em,omitempty"`
}

// TipoTarefaValido verifica se o identificador de tipo de tarefa é reconhecido.
func TipoTarefaValido(tipo string) bool {
	switch tipo {
	case TipoTarefaProgramacao, TipoTarefaArquivo:
		return true
	default:
		return false
	}
}

// NormalizarTipoTarefa devolve "programacao" quando vazio (retrocompatibilidade).
func NormalizarTipoTarefa(tipo string) string {
	t := strings.TrimSpace(strings.ToLower(tipo))
	if t == "" {
		return TipoTarefaProgramacao
	}
	return t
}

// NormalizarExtensao padroniza uma extensão para minúsculas com ponto inicial (ex: "PDF" -> ".pdf").
func NormalizarExtensao(ext string) (string, bool) {
	e := strings.TrimSpace(strings.ToLower(ext))
	if e == "" {
		return "", false
	}
	if !strings.HasPrefix(e, ".") {
		e = "." + e
	}
	if len(e) < 2 || len(e) > 16 {
		return "", false
	}
	for _, r := range e[1:] {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return "", false
		}
	}
	// Bloqueia extensões de executáveis diretos de sistema por segurança
	switch e {
	case ".exe", ".bat", ".cmd", ".com", ".msi", ".scr", ".dll", ".so", ".dylib", ".bin", ".elf":
		return "", false
	}
	return e, true
}

// NormalizarConfigArquivo valida e aplica padrões à configuração de uma tarefa de arquivo.
func NormalizarConfigArquivo(cfg ConfigTarefaArquivo, tetoGlobal int64) (ConfigTarefaArquivo, error) {
	if tetoGlobal <= 0 || tetoGlobal > MaxArquivoMaxBytes {
		tetoGlobal = DefaultArquivoMaxBytes
	}
	maxB := cfg.MaxBytes
	if maxB <= 0 {
		maxB = tetoGlobal
	}
	if maxB < MinArquivoMaxBytes || maxB > tetoGlobal {
		return ConfigTarefaArquivo{}, fmt.Errorf("tamanho máximo deve estar entre %d KB e %d MB",
			MinArquivoMaxBytes/1024, tetoGlobal/(1024*1024))
	}

	vistas := make(map[string]bool)
	var exts []string
	for _, raw := range cfg.ExtensoesPermitidas {
		for _, parte := range strings.Split(raw, ",") {
			if strings.TrimSpace(parte) == "" {
				continue
			}
			norm, ok := NormalizarExtensao(parte)
			if !ok {
				return ConfigTarefaArquivo{}, fmt.Errorf("extensão inválida ou não permitida: %q", parte)
			}
			if !vistas[norm] {
				vistas[norm] = true
				exts = append(exts, norm)
			}
		}
	}
	if len(exts) == 0 {
		exts = []string{".pdf", ".zip", ".txt", ".py", ".c", ".cpp"}
	}
	return ConfigTarefaArquivo{
		MaxBytes:            maxB,
		ExtensoesPermitidas: exts,
	}, nil
}

// SanitizarNomeArquivo limpa caminhos (path traversal), caracteres de controle
// e garante um nome seguro para armazenamento e Content-Disposition.
func SanitizarNomeArquivo(nome string) (string, error) {
	limpo := strings.ReplaceAll(nome, "\\", "/")
	limpo = filepath.Base(limpo)
	limpo = strings.TrimSpace(limpo)
	if limpo == "" || limpo == "." || limpo == ".." || strings.HasPrefix(limpo, ".") {
		return "", ErrArquivoNomeInvalido
	}
	var sb strings.Builder
	for _, r := range limpo {
		if r < 32 || r == 127 || r == '"' || r == '\'' || r == ';' || r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '<' || r == '>' || r == '|' {
			sb.WriteRune('_')
		} else {
			sb.WriteRune(r)
		}
	}
	res := strings.TrimSpace(sb.String())
	if res == "" || len(res) > 255 {
		return "", ErrArquivoNomeInvalido
	}
	return res, nil
}

// MetadadosArquivoValidado contém o resultado da validação completa de um arquivo.
type MetadadosArquivoValidado struct {
	NomeSanitizado string
	Extensao       string
	MimeType       string
	TamanhoBytes   int64
	Sha256         string
}

// ValidarArquivo verifica nome, tamanho, extensão permitida e assinatura binária (magic bytes)
// do conteúdo submetido.
func ValidarArquivo(nome string, dados []byte, cfg ConfigTarefaArquivo, tetoGlobal int64) (MetadadosArquivoValidado, error) {
	if len(dados) == 0 {
		return MetadadosArquivoValidado{}, ErrArquivoVazio
	}
	cfgNorm, err := NormalizarConfigArquivo(cfg, tetoGlobal)
	if err != nil {
		return MetadadosArquivoValidado{}, err
	}
	if int64(len(dados)) > cfgNorm.MaxBytes {
		return MetadadosArquivoValidado{}, ErrArquivoTamanhoExcedido
	}
	nomeSanitizado, err := SanitizarNomeArquivo(nome)
	if err != nil {
		return MetadadosArquivoValidado{}, err
	}
	ext := strings.ToLower(filepath.Ext(nomeSanitizado))
	if ext == "" {
		return MetadadosArquivoValidado{}, ErrArquivoExtensaoNegada
	}
	permitida := false
	for _, p := range cfgNorm.ExtensoesPermitidas {
		if strings.EqualFold(p, ext) {
			permitida = true
			break
		}
	}
	if !permitida {
		return MetadadosArquivoValidado{}, ErrArquivoExtensaoNegada
	}

	// Rejeita binários executáveis (ELF, PE/MZ, Mach-O) independente da extensão
	if pareceExecutavelNat(dados) {
		return MetadadosArquivoValidado{}, ErrArquivoConteudoNegado
	}

	mimeDetectado, ok := validarConteudoPorExtensao(ext, dados)
	if !ok {
		return MetadadosArquivoValidado{}, ErrArquivoConteudoNegado
	}

	sum := sha256.Sum256(dados)
	return MetadadosArquivoValidado{
		NomeSanitizado: nomeSanitizado,
		Extensao:       ext,
		MimeType:       mimeDetectado,
		TamanhoBytes:   int64(len(dados)),
		Sha256:         hex.EncodeToString(sum[:]),
	}, nil
}

func pareceExecutavelNat(dados []byte) bool {
	if len(dados) >= 4 && bytes.Equal(dados[:4], []byte{0x7f, 'E', 'L', 'F'}) {
		return true // ELF (Linux)
	}
	if len(dados) >= 2 && bytes.Equal(dados[:2], []byte{'M', 'Z'}) {
		return true // PE/DOS (Windows .exe/.dll)
	}
	if len(dados) >= 4 {
		// Mach-O magic numbers
		m := dados[:4]
		if bytes.Equal(m, []byte{0xfe, 0xed, 0xfa, 0xce}) ||
			bytes.Equal(m, []byte{0xfe, 0xed, 0xfa, 0xcf}) ||
			bytes.Equal(m, []byte{0xce, 0xfa, 0xed, 0xfe}) ||
			bytes.Equal(m, []byte{0xcf, 0xfa, 0xed, 0xfe}) {
			return true
		}
	}
	return false
}

// validarConteudoPorExtensao cruza a extensão com os magic bytes / texto real.
func validarConteudoPorExtensao(ext string, dados []byte) (string, bool) {
	sniff := http.DetectContentType(dados)

	switch ext {
	case ".pdf":
		if len(dados) >= 5 && bytes.Equal(dados[:5], []byte("%PDF-")) {
			return "application/pdf", true
		}
		return "", false

	case ".zip", ".docx", ".xlsx", ".pptx", ".odt", ".ods", ".odp", ".jar":
		// ZIP normal (PK\x03\x04) ou vazio (PK\x05\x06)
		if len(dados) >= 4 && (bytes.Equal(dados[:4], []byte{'P', 'K', 0x03, 0x04}) || bytes.Equal(dados[:4], []byte{'P', 'K', 0x05, 0x06})) {
			return "application/zip", true
		}
		return "", false

	case ".gz", ".tgz":
		if len(dados) >= 2 && bytes.Equal(dados[:2], []byte{0x1f, 0x8b}) {
			return "application/x-gzip", true
		}
		return "", false

	case ".png":
		if len(dados) >= 8 && bytes.Equal(dados[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
			return "image/png", true
		}
		return "", false

	case ".jpg", ".jpeg":
		if len(dados) >= 3 && bytes.Equal(dados[:3], []byte{0xff, 0xd8, 0xff}) {
			return "image/jpeg", true
		}
		return "", false

	case ".gif":
		if len(dados) >= 6 && (bytes.Equal(dados[:6], []byte("GIF87a")) || bytes.Equal(dados[:6], []byte("GIF89a"))) {
			return "image/gif", true
		}
		return "", false

	case ".txt", ".md", ".csv", ".json", ".xml", ".yaml", ".yml", ".sql",
		".py", ".c", ".cpp", ".cc", ".h", ".hpp", ".java", ".js", ".ts", ".go", ".rs", ".sh", ".html", ".css":
		// Arquivos de código/texto não devem conter bytes nulos e devem ser UTF-8 (ou ASCII/Latin legível sem binário arbitrário)
		amostra := dados
		if len(amostra) > 8192 {
			amostra = amostra[:8192]
		}
		if bytes.IndexByte(amostra, 0x00) != -1 {
			return "", false
		}
		if !utf8.Valid(amostra) {
			// Verifica se ao menos não tem bytes de controle estranhos (permite ISO-8859-1 acentuado comum em comentários)
			for _, b := range amostra {
				if b < 0x09 || (b > 0x0d && b < 0x20) {
					return "", false
				}
			}
		}
		return "text/plain; charset=utf-8", true

	default:
		// Outras extensões customizadas configuradas pelo professor (ex: .ipynb, .r, .mat, .pkt)
		if strings.HasPrefix(sniff, "text/") {
			return "text/plain; charset=utf-8", true
		}
		return "application/octet-stream", true
	}
}
