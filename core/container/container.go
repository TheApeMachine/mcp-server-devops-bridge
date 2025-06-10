package container

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stdcopy"
)

// Container is a wrapper around the Docker client that provides methods for building
// and running a specific container. It encapsulates the complexity of Docker operations.
type Container struct {
	client      *client.Client
	ContainerID string
	ImageName   string
}

// NewContainer creates a new Container manager instance.
// It initializes a Docker client using the host's Docker environment settings.
func NewContainer(imageName string) (*Container, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &Container{client: cli, ImageName: imageName}, nil
}

// BuildImage constructs a Docker image from a Dockerfile in the specified directory.
func (c *Container) BuildImage() error {
	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}

	tar, err := archive.TarWithOptions(currentDir, &archive.TarOptions{})
	if err != nil {
		return err
	}

	opts := types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Context:    tar,
		Tags:       []string{c.ImageName},
		Remove:     false,
	}

	resp, err := c.client.ImageBuild(context.Background(), tar, opts)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// Run creates and starts a new container.
func (c *Container) Run(ctx context.Context, cmd []string, paths []string) error {
	var hostConfig *container.HostConfig
	if len(paths) > 0 {
		bindMounts := []mount.Mount{}
		for _, path := range paths {
			bindMounts = append(bindMounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: path,
				Target: "/tmp" + path,
			})
		}
		hostConfig = &container.HostConfig{Mounts: bindMounts}
	}

	resp, err := c.client.ContainerCreate(ctx, &container.Config{
		Image: c.ImageName,
		Cmd:   cmd,
	}, hostConfig, nil, nil, "")
	if err != nil {
		return err
	}
	c.ContainerID = resp.ID

	if err := c.client.ContainerStart(ctx, c.ContainerID, container.StartOptions{}); err != nil {
		return err
	}

	return nil
}

// Execute executes a command in the running container and returns the output.
func (c *Container) Execute(ctx context.Context, cmd []string) (string, error) {
	if c.ContainerID == "" {
		return "", errors.New("container is not running")
	}

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execIDResp, err := c.client.ContainerExecCreate(ctx, c.ContainerID, execConfig)
	if err != nil {
		return "", err
	}

	execAttachResp, err := c.client.ContainerExecAttach(ctx, execIDResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", err
	}
	defer execAttachResp.Close()

	var stdout, stderr strings.Builder
	_, err = stdcopy.StdCopy(&stdout, &stderr, execAttachResp.Reader)
	if err != nil {
		return "", err
	}

	if stderr.Len() > 0 {
		return stderr.String(), nil
	}
	return stdout.String(), nil
}

// IsRunning checks if the container is currently running.
func (c *Container) IsRunning(ctx context.Context) bool {
	if c.ContainerID == "" {
		return false
	}
	resp, err := c.client.ContainerInspect(ctx, c.ContainerID)
	if err != nil {
		return false
	}
	return resp.State.Running
}

// StopAndRemove terminates and removes the container.
func (c *Container) StopAndRemove(ctx context.Context) error {
	if c.ContainerID == "" {
		return nil // Nothing to do
	}
	// Use a short timeout to prevent hanging
	timeoutSeconds := 5
	if err := c.client.ContainerStop(ctx, c.ContainerID, container.StopOptions{Timeout: &timeoutSeconds}); err != nil {
		log.Warnf("Failed to stop container %s: %v", c.ContainerID, err)
	}

	if err := c.client.ContainerRemove(ctx, c.ContainerID, container.RemoveOptions{Force: true}); err != nil {
		log.Warnf("Failed to remove container %s: %v", c.ContainerID, err)
	}
	c.ContainerID = ""
	return nil
}
