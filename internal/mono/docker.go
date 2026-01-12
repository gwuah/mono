package mono

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
)

func CheckDockerAvailable() error {
	cmd := exec.Command("docker", "info")
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := strings.ToLower(string(output))
		if strings.Contains(outputStr, "cannot connect") ||
			strings.Contains(outputStr, "is the docker daemon running") ||
			strings.Contains(outputStr, "connection refused") {
			return fmt.Errorf("docker daemon isn't running, please (re)start it")
		}
		return fmt.Errorf("docker unavailable: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

var composeFilenames = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

func DetectComposeFile(dir string) (string, error) {
	for _, name := range composeFilenames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("no compose file found (tried: %v)", composeFilenames)
}

type ComposeConfig struct {
	project *types.Project
}

func ParseComposeConfig(workDir string) (*ComposeConfig, error) {
	filename, err := DetectComposeFile(workDir)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(workDir, filename))
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file: %w", err)
	}

	configDetails := types.ConfigDetails{
		WorkingDir:  workDir,
		Environment: types.NewMapping(os.Environ()),
		ConfigFiles: []types.ConfigFile{
			{
				Filename: filename,
				Content:  data,
			},
		},
	}

	project, err := loader.LoadWithContext(context.Background(), configDetails,
		func(o *loader.Options) {
			o.SetProjectName(filepath.Base(workDir), false)
			o.SkipValidation = true
			o.SkipResolveEnvironment = true
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse compose config: %w", err)
	}

	return &ComposeConfig{project: project}, nil
}

func (c *ComposeConfig) GetServicePorts() map[string][]int {
	result := make(map[string][]int)
	for _, svc := range c.project.Services {
		var ports []int
		for _, p := range svc.Ports {
			if p.Target > 0 {
				ports = append(ports, int(p.Target))
			}
		}
		if len(ports) > 0 {
			result[svc.Name] = ports
		}
	}
	return result
}

func (c *ComposeConfig) GetServiceNames() []string {
	names := make([]string, 0, len(c.project.Services))
	for _, svc := range c.project.Services {
		names = append(names, svc.Name)
	}
	return names
}

func (c *ComposeConfig) Project() *types.Project {
	return c.project
}

func ApplyOverrides(project *types.Project, envName string, allocations []Allocation) {
	monoPrefix := fmt.Sprintf("mono-%s", envName)

	portsByService := make(map[string][]types.ServicePortConfig)
	for _, alloc := range allocations {
		portsByService[alloc.Service] = append(portsByService[alloc.Service], types.ServicePortConfig{
			Target:    uint32(alloc.ContainerPort),
			Published: fmt.Sprintf("%d", alloc.HostPort),
		})
	}

	for name, svc := range project.Services {
		if newPorts, ok := portsByService[name]; ok {
			svc.Ports = newPorts
			project.Services[name] = svc
		}
	}

	project.Networks = types.Networks{
		"default": types.NetworkConfig{
			Name: monoPrefix,
		},
	}

	newVolumes := types.Volumes{}
	for volName, volConfig := range project.Volumes {
		volConfig.Name = fmt.Sprintf("%s_%s", monoPrefix, volName)
		newVolumes[volName] = volConfig
	}
	project.Volumes = newVolumes
}

func WriteComposeOverride(path string, project *types.Project) error {
	data, err := project.MarshalYAML()
	if err != nil {
		return fmt.Errorf("failed to marshal project: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

func StartContainers(projectName, workDir string, stdout, stderr io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "compose",
		"-p", projectName,
		"-f", "docker-compose.mono.yml",
		"up", "-d")
	cmd.Dir = workDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("docker compose up timed out")
		}
		return fmt.Errorf("failed to start containers: %w", err)
	}
	return nil
}

func StopContainers(projectName, workDir string, removeVolumes bool, stdout, stderr io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	args := []string{"compose", "-p", projectName, "down"}
	if removeVolumes {
		args = append(args, "-v")
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("docker compose down timed out")
		}
		return fmt.Errorf("failed to stop containers: %w", err)
	}
	return nil
}

func ContainersRunning(projectName string) bool {
	cmd := exec.Command("docker", "compose", "-p", projectName, "ps", "-q")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}
