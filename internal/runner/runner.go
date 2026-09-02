// Package runner executa código de submissões em containers efêmeros
// isolados: sem rede, memória/CPU/pids limitados, privilégios mínimos.
// O código e a entrada são injetados via CopyToContainer (sem bind mounts),
// o que funciona com a API no host (dev) ou em container (produção).
package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	"seno/internal/services"
)

const (
	maxOutputBytes   = 64 * 1024
	memoryLimit      = 128 * 1024 * 1024
	cpuLimit         = 1e9
	pidsLimit        = 64
	runUserID        = "65534:65534"
	imagePullTimeout = 5 * time.Minute
	targetDir        = "/tmp"
)

type languageSpec struct {
	image   string
	command []string
	timeout time.Duration
}

var languageSpecs = map[string]languageSpec{
	"python": {
		image:   "python:3.12-alpine",
		command: []string{"sh", "-c", "python3 /tmp/main.py < /tmp/input.txt"},
		timeout: 10 * time.Second,
	},
	"c": {
		image:   "gcc:13-alpine",
		command: []string{"sh", "-c", "cc -O2 -o /tmp/prog /tmp/main.c && /tmp/prog < /tmp/input.txt"},
		timeout: 30 * time.Second,
	},
	"cpp": {
		image:   "gcc:13-alpine",
		command: []string{"sh", "-c", "c++ -O2 -o /tmp/prog /tmp/main.cpp && /tmp/prog < /tmp/input.txt"},
		timeout: 30 * time.Second,
	},
}

func sourceFilename(language string) string {
	switch language {
	case "python":
		return "main.py"
	case "c":
		return "main.c"
	case "cpp":
		return "main.cpp"
	default:
		return "main.txt"
	}
}

type Runner struct {
	cli *client.Client
}

// NewRunner conecta ao daemon Docker (DOCKER_HOST via FromEnv: pipe do
// Docker Desktop no dev, unix socket em produção).
func NewRunner() (*Runner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao Docker: %w", err)
	}
	return &Runner{cli: cli}, nil
}

// Run executa o código contra uma entrada e devolve stdout/stderr.
// Timeout estourado: TimedOut=true e o container é morto.
func (r *Runner) Run(ctx context.Context, req services.RunRequest) (services.RunResult, error) {
	spec, ok := languageSpecs[req.Language]
	if !ok {
		return services.RunResult{}, services.ErrUnsupportedLanguage
	}

	ctx, cancel := context.WithTimeout(ctx, spec.timeout)
	defer cancel()

	if err := r.ensureImage(ctx, spec.image); err != nil {
		return services.RunResult{}, err
	}

	var pids int64 = pidsLimit
	cfg := &container.Config{
		Image: spec.image,
		Cmd:   spec.command,
		User:  runUserID,
	}
	hostCfg := &container.HostConfig{
		NetworkMode: "none",
		Resources: container.Resources{
			Memory:     memoryLimit,
			MemorySwap: memoryLimit,
			NanoCPUs:   cpuLimit,
			PidsLimit:  &pids,
		},
		CapDrop:     []string{"ALL"},
		SecurityOpt: []string{"no-new-privileges"},
	}

	createResp, err := r.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     cfg,
		HostConfig: hostCfg,
	})
	if err != nil {
		return services.RunResult{}, fmt.Errorf("erro ao criar container de execução: %w", err)
	}
	containerID := createResp.ID
	defer func() {
		_, _ = r.cli.ContainerRemove(context.Background(), containerID, client.ContainerRemoveOptions{Force: true})
	}()

	if err := r.injectFiles(ctx, containerID, sourceFilename(req.Language), req); err != nil {
		return services.RunResult{}, err
	}

	start := time.Now()
	if _, err := r.cli.ContainerStart(ctx, containerID, client.ContainerStartOptions{}); err != nil {
		return services.RunResult{}, fmt.Errorf("erro ao iniciar container de execução: %w", err)
	}

	timedOut := false
	exitCode := 0
	waitResult := r.cli.ContainerWait(ctx, containerID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})
	select {
	case <-ctx.Done():
		timedOut = true
		_, _ = r.cli.ContainerKill(context.Background(), containerID, client.ContainerKillOptions{Signal: "KILL"})
	case waitErr := <-waitResult.Error:
		if waitErr != nil {
			return services.RunResult{}, fmt.Errorf("erro ao aguardar execução: %w", waitErr)
		}
	case status := <-waitResult.Result:
		exitCode = int(status.StatusCode)
	}

	stdout, stderr, err := r.readLogs(ctx, containerID)
	if err != nil {
		return services.RunResult{}, err
	}

	return services.RunResult{
		Stdout:     stdout,
		Stderr:     stderr,
		ExitCode:   exitCode,
		TimedOut:   timedOut,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// ensureImage garante que a imagem exista localmente, baixando-a no
// primeiro uso (pull fora do timeout curto da execução).
func (r *Runner) ensureImage(ctx context.Context, ref string) error {
	_, err := r.cli.ImageInspect(ctx, ref)
	if err == nil {
		return nil
	}
	if !errdefs.IsNotFound(err) {
		return fmt.Errorf("erro ao inspecionar imagem %s: %w", ref, err)
	}

	pullCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), imagePullTimeout)
	defer cancel()

	resp, err := r.cli.ImagePull(pullCtx, ref, client.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("erro ao baixar imagem %s: %w", ref, err)
	}
	// Wait aguarda o pull completar; Close não é necessário com Wait.
	if err := resp.Wait(pullCtx); err != nil {
		return fmt.Errorf("erro ao baixar imagem %s: %w", ref, err)
	}
	return nil
}

// injectFiles copia fonte e entrada para o container em um único tar
// (modo 0444: legível pelo usuário de execução).
func (r *Runner) injectFiles(ctx context.Context, containerID, filename string, req services.RunRequest) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	files := []struct {
		name string
		body string
	}{
		{name: filename, body: req.SourceCode},
		{name: "input.txt", body: req.Stdin},
	}
	for _, f := range files {
		hdr := &tar.Header{Name: f.name, Mode: 0o444, Size: int64(len(f.body))}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("erro ao montar arquivo de execução: %w", err)
		}
		if _, err := tw.Write([]byte(f.body)); err != nil {
			return fmt.Errorf("erro ao montar arquivo de execução: %w", err)
		}
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("erro ao montar arquivo de execução: %w", err)
	}

	if _, err := r.cli.CopyToContainer(ctx, containerID, client.CopyToContainerOptions{
		Content:                   &buf,
		DestinationPath:           targetDir,
		AllowOverwriteDirWithFile: true,
	}); err != nil {
		return fmt.Errorf("erro ao injetar arquivos no container: %w", err)
	}
	return nil
}

// readLogs captura stdout e stderr (demultiplexados), cada um truncado em
// maxOutputBytes. Usa ctx sem cancelamento: o timeout pode ter estourado,
// mas os logs continuam legíveis.
func (r *Runner) readLogs(ctx context.Context, containerID string) (string, string, error) {
	logsCtx := context.WithoutCancel(ctx)

	logsResult, err := r.cli.ContainerLogs(logsCtx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", "", fmt.Errorf("erro ao ler saída da execução: %w", err)
	}
	defer logsResult.Close()

	stdout := &limitWriter{}
	stderr := &limitWriter{}
	if _, err := stdcopy.StdCopy(stdout, stderr, logsResult); err != nil {
		return "", "", fmt.Errorf("erro ao decodificar saída da execução: %w", err)
	}

	return stdout.String(), stderr.String(), nil
}

// limitWriter acumula até maxOutputBytes e descarta o excedente.
type limitWriter struct {
	buf bytes.Buffer
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if w.buf.Len() >= maxOutputBytes {
		return len(p), nil
	}
	remaining := maxOutputBytes - w.buf.Len()
	if len(p) > remaining {
		p = p[:remaining]
	}
	_, _ = w.buf.Write(p)
	return len(p), nil
}

func (w *limitWriter) String() string {
	return w.buf.String()
}
