package container

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stdcopy"
)

var once sync.Once
var containerInstance *Container

/*
Builder is a wrapper around the Docker client that provides methods for building
and running containers. It encapsulates the complexity of Docker operations,
allowing for easier management of containerized environments.
*/
type Container struct {
	client      *client.Client
	containerID string
}

/*
NewContainer creates a new Container instance.
It initializes a Docker client using the host's Docker environment settings.
*/
func NewContainer() *Container {
	once.Do(func() {
		cli, err := client.NewClientWithOpts(client.FromEnv)

		if err != nil {
			log.Error(err)
		}

		containerInstance = &Container{client: cli}
	})

	return containerInstance
}

/*
BuildImage constructs a Docker image from a Dockerfile in the specified directory.
This method abstracts the image building process, handling the creation of a tar archive
and the configuration of build options.
*/
func (container *Container) BuildImage() error {
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
		Tags:       []string{"agent"},
		Remove:     false,
		// Add these options for better compatibility:
		BuildArgs: map[string]*string{
			"TARGETARCH": nil, // This will use the default architecture
		},
	}

	resp, err := container.client.ImageBuild(context.Background(), tar, opts)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

/*
RunContainer creates, starts, and attaches to a new container based on the specified image.
It provides channels for stdin and stdout/stderr, enabling interactive communication with the container.
This method is particularly useful for integrating with language models or other interactive processes.

Parameters:
  - ctx: The context for the Docker API calls
  - imageName: The name of the Docker image to use
  - cmd: The command to run in the container
  - username: The username to create and use within the container
  - customMessage: A message to be displayed when attaching to the container

Returns:
  - in: A channel for sending input to the container
  - out: A channel for receiving output from the container
  - err: Any error encountered during the process
*/
func (c *Container) RunContainer(paths []string) error {
	// Create host config with volume mount
	bindMounts := []mount.Mount{}

	for _, path := range paths {
		bindMounts = append(bindMounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: path,
			Target: "/tmp" + path,
		})
	}

	hostConfig := &container.HostConfig{
		Mounts: bindMounts,
	}

	// Create the container with specific configuration
	resp, err := c.client.ContainerCreate(context.Background(), &container.Config{
		Hostname:  "agent",
		User:      "agent",
		Image:     "agent",
		Cmd:       []string{"/bin/sh"},
		Tty:       true,
		OpenStdin: true,
		StdinOnce: false,
		Env: []string{
			fmt.Sprintf("USERNAME=%s", "agent"),
		},
		WorkingDir: "/tmp/workspace",
	}, hostConfig, nil, nil, "")
	if err != nil {
		return err
	}

	// Start the container
	if err := c.client.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
		return err
	}

	return nil
}

/*
ExecuteCommand executes a command in the container and returns the output.
*/
func (c *Container) ExecuteCommand(ctx context.Context, cmd []string) []byte {
	// Join command parts into a single string for shell execution
	commandStr := strings.Join(cmd, " ")
	fullCmd := []string{"/bin/sh", "-c", commandStr}

	// Set up the exec configuration
	execConfig := container.ExecOptions{
		Cmd:          fullCmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	// Create the exec instance
	execIDResp, err := containerInstance.client.ContainerExecCreate(ctx, containerInstance.containerID, execConfig)
	if err != nil {
		return nil
	}

	// Attach to the exec instance
	execAttachResp, err := c.client.ContainerExecAttach(ctx, execIDResp.ID, container.ExecStartOptions{})
	if err != nil {
		return nil
	}
	defer execAttachResp.Close()

	// Read the output
	var stdout, stderr strings.Builder

	_, err = stdcopy.StdCopy(&stdout, &stderr, execAttachResp.Reader)
	if err != nil {
		log.Error(err)
		return nil
	}

	return []byte(stdout.String() + stderr.String())
}

func (c *Container) IsRunning() bool {
	container, err := c.client.ContainerInspect(context.Background(), c.containerID)

	if err != nil {
		return false
	}

	return container.State.Status == "running"
}

// RunCommand runs a command in a container and returns stdout, stderr, and error
func RunCommand(cmd string, paths string) (string, string, error) {
	// Split the paths string into a slice
	var pathsList []string
	if paths != "" {
		pathsList = strings.Split(paths, ",")
		// Trim whitespace from each path
		for i, path := range pathsList {
			pathsList[i] = strings.TrimSpace(path)
		}
	}

	// Create and run container if needed
	container := NewContainer()
	if !container.IsRunning() {
		container.BuildImage()
		container.RunContainer(pathsList)
	}

	// Split the command into parts
	cmdParts := strings.Fields(cmd)
	output := container.ExecuteCommand(context.Background(), cmdParts)

	// For now we don't have separate stdout and stderr, so return output for both
	return string(output), "", nil
}
