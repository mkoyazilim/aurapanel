package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	dockerListTimeout   = 45 * time.Second
	dockerActionTimeout = 90 * time.Second
	dockerPullTimeout   = 15 * time.Minute
)

func containerRuntimeCommand() (string, error) {
	for _, candidate := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("docker or podman not found")
}

func containerRuntimeOutputTrimmed(timeout time.Duration, command string, args ...string) (string, error) {
	output, err := runCommandCombinedOutputWithTimeout(timeout, command, args...)
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed != "" {
			return "", fmt.Errorf("%s", trimmed)
		}
		return "", err
	}
	return trimmed, nil
}

func runtimeDockerContainers() ([]DockerContainer, error) {
	command, err := containerRuntimeCommand()
	if err != nil {
		return nil, err
	}
	output, err := containerRuntimeOutputTrimmed(dockerListTimeout, command, "ps", "-a", "--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}\t{{.RunningFor}}")
	if err != nil {
		// Older runtime templates can miss RunningFor; retry with a smaller compatible format.
		output, err = containerRuntimeOutputTrimmed(dockerListTimeout, command, "ps", "-a", "--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}")
	}
	if err != nil && strings.TrimSpace(err.Error()) == "" {
		return []DockerContainer{}, nil
	}
	if err != nil {
		return nil, err
	}
	containers := []DockerContainer{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		created := ""
		if len(fields) > 5 {
			created = fields[5]
		}
		containers = append(containers, DockerContainer{
			ID:      fields[0],
			Name:    fields[1],
			Image:   fields[2],
			Status:  fields[3],
			Ports:   fields[4],
			Created: created,
		})
	}
	return containers, nil
}

func runtimeDockerImages() ([]DockerImage, error) {
	command, err := containerRuntimeCommand()
	if err != nil {
		return nil, err
	}
	output, err := containerRuntimeOutputTrimmed(dockerListTimeout, command, "images", "--format", "{{.ID}}\t{{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}")
	if err != nil {
		// Fallback for runtimes that do not expose CreatedSince in templates.
		output, err = containerRuntimeOutputTrimmed(dockerListTimeout, command, "images", "--format", "{{.ID}}\t{{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}")
	}
	if err != nil && strings.TrimSpace(err.Error()) == "" {
		return []DockerImage{}, nil
	}
	if err != nil {
		return nil, err
	}
	images := []DockerImage{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}
		created := ""
		if len(fields) > 4 {
			created = fields[4]
		}
		images = append(images, DockerImage{
			ID:         fields[0],
			Repository: fields[1],
			Tag:        fields[2],
			Size:       fields[3],
			Created:    created,
		})
	}
	return images, nil
}

func createRuntimeDockerContainer(name, image string, ports []string, restartPolicy string, memoryLimit string, cpuLimit string, env []string, volumes []string) error {
	command, err := containerRuntimeCommand()
	if err != nil {
		return err
	}
	name = sanitizeName(name)
	if name == "" {
		return fmt.Errorf("container name is required")
	}
	if _, err := containerRuntimeOutputTrimmed(dockerPullTimeout, command, "pull", image); err != nil {
		return err
	}
	args := []string{"run", "-d", "--name", name}
	if strings.TrimSpace(restartPolicy) != "" {
		args = append(args, "--restart", strings.TrimSpace(restartPolicy))
	}
	if strings.TrimSpace(memoryLimit) != "" {
		args = append(args, "-m", strings.TrimSpace(memoryLimit))
	}
	if strings.TrimSpace(cpuLimit) != "" {
		args = append(args, "--cpus", strings.TrimSpace(cpuLimit))
	}
	for _, envVar := range env {
		envVar = strings.TrimSpace(envVar)
		if envVar != "" {
			args = append(args, "-e", envVar)
		}
	}
	for _, volume := range volumes {
		volume = strings.TrimSpace(volume)
		if volume != "" {
			args = append(args, "-v", volume)
		}
	}
	for _, port := range ports {
		port = strings.TrimSpace(port)
		if port != "" {
			args = append(args, "-p", port)
		}
	}
	args = append(args, image)
	_, err = containerRuntimeOutputTrimmed(dockerActionTimeout, command, args...)
	return err
}

func applyRuntimeDockerContainerAction(id, action string) error {
	command, err := containerRuntimeCommand()
	if err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("container id is required")
	}
	switch action {
	case "start", "stop", "restart":
		_, err = containerRuntimeOutputTrimmed(dockerActionTimeout, command, action, id)
	case "remove":
		_, err = containerRuntimeOutputTrimmed(dockerActionTimeout, command, "rm", "-f", id)
	default:
		return fmt.Errorf("unsupported container action")
	}
	return err
}

func pullRuntimeDockerImage(image, tag string) error {
	command, err := containerRuntimeCommand()
	if err != nil {
		return err
	}
	image = strings.TrimSpace(image)
	tag = firstNonEmpty(strings.TrimSpace(tag), "latest")
	if image == "" {
		return fmt.Errorf("image is required")
	}
	_, err = containerRuntimeOutputTrimmed(dockerPullTimeout, command, "pull", image+":"+tag)
	return err
}

func removeRuntimeDockerImage(id string) error {
	command, err := containerRuntimeCommand()
	if err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("image id is required")
	}
	_, err = containerRuntimeOutputTrimmed(dockerActionTimeout, command, "rmi", "-f", id)
	return err
}

type DockerFileEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
	Mode  string `json:"mode"`
}

func runtimeDockerContainerFiles(containerID, path string) ([]DockerFileEntry, error) {
	command, err := containerRuntimeCommand()
	if err != nil {
		return nil, err
	}
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return nil, fmt.Errorf("container id is required")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/"
	}
	// List files with detailed info using ls -la (compatible with BusyBox)
	output, err := containerRuntimeOutputTrimmed(30*time.Second, command, "exec", containerID, "ls", "-la", path)
	if err != nil {
		return nil, fmt.Errorf("failed to list files in container: %w", err)
	}
	entries := make([]DockerFileEntry, 0)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		permissions := fields[0]
		name := fields[6]
		// Handle names with spaces - join remaining fields
		if len(fields) > 7 {
			name = strings.Join(fields[6:], " ")
		}
		// Skip . and .. entries
		if name == "." || name == ".." {
			continue
		}
		isDir := permissions[0] == 'd'
		var size int64
		if !isDir && len(fields) >= 5 {
			fmt.Sscanf(fields[4], "%d", &size)
		}
		entries = append(entries, DockerFileEntry{
			Name:  name,
			IsDir: isDir,
			Size:  size,
			Mode:  permissions,
		})
	}
	return entries, nil
}

func runtimeDockerContainerFileContent(containerID, filePath string) (string, error) {
	command, err := containerRuntimeCommand()
	if err != nil {
		return "", err
	}
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return "", fmt.Errorf("container id is required")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return "", fmt.Errorf("file path is required")
	}
	output, err := containerRuntimeOutputTrimmed(30*time.Second, command, "exec", containerID, "cat", filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return output, nil
}
