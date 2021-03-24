// +build integration

package application

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	dockerFilters "github.com/docker/docker/api/types/filters"
	dockerNetwork "github.com/docker/docker/api/types/network"
	dockerClient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
)

func TestDockerMain(t *testing.T) {
	container := runContainer(t)
	defer removeContainer(t, container)

	CaseGetBeforePost(t)
	CasePostBeforeGet(t)
	CaseGetTimeout(t)
	CaseServiceTerminating(t, container)
}

func TestDockerDoubleTerm(t *testing.T) {
	container := runContainer(t)
	defer removeContainer(t, container)

	CaseServiceDoubleTerminating(t, container)
}

func CaseGetBeforePost(t *testing.T) {
	getDone := make(chan bool)
	go func() {
		resp, err := http.Get("http://localhost:2308/")
		assert.Nil(t, err)
		if err == nil {
			assert.Equal(t, 200, resp.StatusCode)
		}
		getDone <- true
	}()
	resp, err := http.Post("http://localhost:2308/", "text/plain", nil)
	assert.Nil(t, err)
	if err == nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	select {
	case <-getDone:
	case <-time.After(time.Second):
		t.Error("GET is stuck")
	}
}

func CasePostBeforeGet(t *testing.T) {
	resp, err := http.Post("http://localhost:2308/", "text/plain", bytes.NewReader([]byte("FOOBAR")))
	assert.Nil(t, err)
	if err == nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	resp, err = http.Get("http://localhost:2308/")
	assert.Nil(t, err)
	if err == nil {
		assert.Equal(t, 200, resp.StatusCode)
		defer resp.Body.Close()
		b, _ := ioutil.ReadAll(resp.Body)
		assert.Equal(t, []byte("FOOBAR\n"), b)
	}
}

func CaseGetTimeout(t *testing.T) {
	resp, err := http.Get("http://localhost:2308/")

	assert.Nil(t, err)
	if err == nil {
		defer resp.Body.Close()
		b, _ := ioutil.ReadAll(resp.Body)

		assert.Equal(t, 500, resp.StatusCode)
		assert.Equal(t, []byte("Timeout exceeded\n"), b)
	}
}

// The goal of this case is to test that
// * we can POST while the service is running
// * we cannot POST while the service is terminating
// * we can GET while the service is terminating and the queue is not empty
// * the service is terminated as soon as the queue is empty
func CaseServiceTerminating(t *testing.T, container *dockerContainer.ContainerCreateCreatedBody) {
	resp, err := http.Post("http://localhost:2308/", "text/plain", nil)
	assert.Nil(t, err)
	if err == nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	termContainer(t, container)

	resp, err = http.Post("http://localhost:2308/", "text/plain", nil)
	assert.Nil(t, err)
	if err == nil {
		assert.Equal(t, 500, resp.StatusCode)
	}

	resp, err = http.Get("http://localhost:2308/")
	assert.Nil(t, err)
	if err == nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	checkContainer(t, container, false, 5)
}

// The goal of this case is to test that
// * we can POST while the service is running
// * the service is terminated upon receiving the second SIGTERM even though the queue is NOT empty
func CaseServiceDoubleTerminating(t *testing.T, container *dockerContainer.ContainerCreateCreatedBody) {
	resp, err := http.Post("http://localhost:2308/", "text/plain", nil)
	assert.Nil(t, err)
	if err == nil {
		assert.Equal(t, 200, resp.StatusCode)
	}

	termContainer(t, container)
	termContainer(t, container)

	checkContainer(t, container, false, 5)
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

	checkContainer(t, &container, true, 0)

	return &container
}

func termContainer(t *testing.T, container *dockerContainer.ContainerCreateCreatedBody) {
	client, err := dockerClient.NewEnvClient()
	if err != nil {
		t.Fatalf("Cannot connect to Docker daemon: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer func() {
		cancel()
	}()

	err = client.ContainerKill(ctx, container.ID, "SIGTERM")
	if err != nil {
		t.Fatalf("cannot send signal to Docker container: %s", err)
	}
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

func checkContainer(
	t *testing.T,
	container *dockerContainer.ContainerCreateCreatedBody,
	running bool,
	retries int,
) {
	var expectedNumberOfContainers int
	if running {
		expectedNumberOfContainers = 1
	} else {
		expectedNumberOfContainers = 0
	}
	actualNumberOfContainers := -1

	client, err := dockerClient.NewEnvClient()
	if err != nil {
		t.Fatalf("Cannot connect to Docker daemon: %s", err)
	}

	filters := dockerFilters.NewArgs()
	filters.Add("id", container.ID)

	for i := 0; i <= retries; i++ {
		containers, err := client.ContainerList(context.Background(), dockerTypes.ContainerListOptions{
			Filters: filters,
		})
		assert.Nil(t, err)
		if err == nil {
			actualNumberOfContainers = len(containers)
		}

		if actualNumberOfContainers == expectedNumberOfContainers {
			break
		}

		time.Sleep(time.Second)
	}

	assert.Equal(t, expectedNumberOfContainers, actualNumberOfContainers)
}
