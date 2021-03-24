// +build integration

package application

import (
	"context"
	"net/http"
	"testing"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	dockerNetwork "github.com/docker/docker/api/types/network"
	dockerClient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
)

func TestDocker(t *testing.T) {
	container := runContainer(t)
	defer removeContainer(t, container)

	done := make(chan bool)
	go func() {
		resp, err := http.Get("http://localhost:2308/")
		assert.Nil(t, err)
		if err == nil {
			assert.Equal(t, 200, resp.StatusCode)
		}
		done <- true
	}()
	go func() {
		resp, err := http.Post("http://localhost:2308/", "text/plain", nil)
		assert.Nil(t, err)
		if err == nil {
			assert.Equal(t, 200, resp.StatusCode)
		}
		done <- true
	}()

	<-done
	<-done
}

func removeContainer(t *testing.T, container *dockerContainer.ContainerCreateCreatedBody) {
	client, err := dockerClient.NewEnvClient()
	if err != nil {
		t.Fatalf("Cannot connect to Docker daemon: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer func() {
		cancel()
	}()

	err = client.ContainerStop(ctx, container.ID, nil)
	if err != nil {
		t.Fatalf("cannot remove Docker container: %s", err)
	}
}

func runContainer(t *testing.T) *dockerContainer.ContainerCreateCreatedBody {
	client, err := dockerClient.NewEnvClient()
	if err != nil {
		t.Fatalf("Cannot connect to Docker daemon: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer func() {
		cancel()
	}()

	container, err := client.ContainerCreate(
		ctx,
		&dockerContainer.Config{
			Image:        "passive_balancer",
			ExposedPorts: nat.PortSet{"2308": {}},
		},
		&dockerContainer.HostConfig{PortBindings: nat.PortMap{
			"2308": {{HostPort: "2308"}},
		}},
		&dockerNetwork.NetworkingConfig{},
		"",
	)
	if err != nil {
		t.Fatalf("Cannot create Docker container: %s", err)
	}

	err = client.ContainerStart(ctx, container.ID, dockerTypes.ContainerStartOptions{})
	if err != nil {
		t.Fatalf("Cannot start Docker container: %s", err)
	}

	return &container
}
