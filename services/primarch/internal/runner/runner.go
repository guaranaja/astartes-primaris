// Package runner executes marine strategies via different backends.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// Result holds the outcome of a marine execution cycle.
type Result struct {
	SignalsGenerated int
	OrdersSubmitted  int
	Output           string
	Duration         time.Duration
}

// Manager handles executing marines via their configured runner type.
type Manager struct {
	logger *slog.Logger
}

// NewManager creates a runner manager.
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{logger: logger}
}

// Execute runs a marine's strategy and returns the result.
func (rm *Manager) Execute(ctx context.Context, m *domain.Marine) (*Result, error) {
	timeout := 30 * time.Second
	if m.Resources.TimeoutSeconds > 0 {
		timeout = time.Duration(m.Resources.TimeoutSeconds) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch m.RunnerType {
	case domain.RunnerProcess:
		return rm.executeProcess(ctx, m)
	case domain.RunnerDocker:
		return rm.executeDocker(ctx, m)
	case domain.RunnerRemote:
		return rm.executeRemote(ctx, m)
	default:
		// Default to process runner for simplicity
		return rm.executeProcess(ctx, m)
	}
}

// executeProcess runs a strategy as a local process.
// This is the primary integration path for astartes-futures strategies.
func (rm *Manager) executeProcess(ctx context.Context, m *domain.Marine) (*Result, error) {
	if m.RunnerConfig.Command == "" {
		return nil, fmt.Errorf("process runner requires a command")
	}

	start := time.Now()

	cmd := exec.CommandContext(ctx, m.RunnerConfig.Command, m.RunnerConfig.Args...)
	if m.RunnerConfig.WorkDir != "" {
		cmd.Dir = m.RunnerConfig.WorkDir
	}

	// Set environment variables
	for k, v := range m.RunnerConfig.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Inject marine identity into env
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("MARINE_ID=%s", m.ID),
		fmt.Sprintf("MARINE_NAME=%s", m.Name),
		fmt.Sprintf("STRATEGY_NAME=%s", m.StrategyName),
		fmt.Sprintf("BROKER_ACCOUNT=%s", m.BrokerAccountID),
	)

	// Inject strategy parameters
	for k, v := range m.Parameters {
		cmd.Env = append(cmd.Env, fmt.Sprintf("PARAM_%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	rm.logger.Info("executing process", "marine", m.ID, "command", m.RunnerConfig.Command,
		"args", m.RunnerConfig.Args, "workdir", m.RunnerConfig.WorkDir)

	err := cmd.Run()

	result := &Result{
		Output:   stdout.String(),
		Duration: time.Since(start),
	}

	if err != nil {
		return result, fmt.Errorf("process failed: %w\nstderr: %s", err, stderr.String())
	}

	rm.logger.Info("process complete", "marine", m.ID, "duration", result.Duration,
		"output_len", len(result.Output))

	return result, nil
}

// executeDocker runs a strategy in a Docker container.
func (rm *Manager) executeDocker(ctx context.Context, m *domain.Marine) (*Result, error) {
	if m.RunnerConfig.Image == "" {
		return nil, fmt.Errorf("docker runner requires an image")
	}

	start := time.Now()

	args := []string{
		"run", "--rm",
		"--name", fmt.Sprintf("marine-%s", m.ID),
	}

	// Memory limit
	if m.Resources.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", m.Resources.MemoryMB))
	}

	// CPU limit
	if m.Resources.CPUMillicores > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", float64(m.Resources.CPUMillicores)/1000))
	}

	// Network
	if m.RunnerConfig.Network != "" {
		args = append(args, "--network", m.RunnerConfig.Network)
	}

	// Environment
	for k, v := range m.RunnerConfig.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args,
		"-e", fmt.Sprintf("MARINE_ID=%s", m.ID),
		"-e", fmt.Sprintf("MARINE_NAME=%s", m.Name),
		"-e", fmt.Sprintf("STRATEGY_NAME=%s", m.StrategyName),
		"-e", fmt.Sprintf("BROKER_ACCOUNT=%s", m.BrokerAccountID),
	)

	for k, v := range m.Parameters {
		args = append(args, "-e", fmt.Sprintf("PARAM_%s=%s", k, v))
	}

	// Volumes
	for _, v := range m.RunnerConfig.Volumes {
		args = append(args, "-v", v)
	}

	args = append(args, m.RunnerConfig.Image)

	cmd := exec.CommandContext(ctx, "docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	rm.logger.Info("executing docker", "marine", m.ID, "image", m.RunnerConfig.Image)

	err := cmd.Run()

	result := &Result{
		Output:   stdout.String(),
		Duration: time.Since(start),
	}

	if err != nil {
		return result, fmt.Errorf("docker run failed: %w\nstderr: %s", err, stderr.String())
	}

	return result, nil
}

// executeRemote calls an already-running strategy service via HTTP.
func (rm *Manager) executeRemote(ctx context.Context, m *domain.Marine) (*Result, error) {
	if m.RunnerConfig.Endpoint == "" {
		return nil, fmt.Errorf("remote runner requires an endpoint")
	}

	start := time.Now()

	// Trigger execution via POST
	url := fmt.Sprintf("%s/execute", m.RunnerConfig.Endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Marine-ID", m.ID)
	req.Header.Set("X-Strategy-Name", m.StrategyName)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote execution failed: %w", err)
	}
	defer resp.Body.Close()

	result := &Result{
		Duration: time.Since(start),
	}

	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("remote execution returned %d", resp.StatusCode)
	}

	return result, nil
}
