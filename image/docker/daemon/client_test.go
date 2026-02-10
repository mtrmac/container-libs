package daemon

import (
	"net/http"
	"path/filepath"
	"testing"

	dockerclient "github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/image/v5/types"
)

func TestDockerClientFromNilSystemContext(t *testing.T) {
	client, err := newDockerClient(nil)

	assert.Nil(t, err, "There should be no error creating the Docker client")
	assert.NotNil(t, client, "A Docker client reference should have been returned")

	assert.Equal(t, dockerclient.DefaultDockerHost, client.DaemonHost(), "The default docker host should have been used")

	assert.NoError(t, client.Close())
}

func TestDockerClientFromCertContext(t *testing.T) {
	host := "tcp://127.0.0.1:2376"
	systemCtx := &types.SystemContext{
		DockerDaemonCertPath:              filepath.Join("testdata", "certs"),
		DockerDaemonHost:                  host,
		DockerDaemonInsecureSkipTLSVerify: true,
	}

	client, err := newDockerClient(systemCtx)

	assert.Nil(t, err, "There should be no error creating the Docker client")
	assert.NotNil(t, client, "A Docker client reference should have been returned")

	assert.Equal(t, host, client.DaemonHost())

	assert.NoError(t, client.Close())
}

func TestTlsConfig(t *testing.T) {
	tests := []struct {
		ctx          *types.SystemContext
		wantInsecure bool
		wantCerts    int
	}{
		{&types.SystemContext{
			DockerDaemonCertPath:              filepath.Join("testdata", "certs"),
			DockerDaemonInsecureSkipTLSVerify: true,
		}, true, 1},
		{&types.SystemContext{DockerDaemonInsecureSkipTLSVerify: true}, true, 0},
	}
	for _, c := range tests {
		httpClient, err := tlsConfig(c.ctx)
		require.NoError(t, err)
		tlsCfg := httpClient.Transport.(*http.Transport).TLSClientConfig
		assert.Equal(t, c.wantInsecure, tlsCfg.InsecureSkipVerify)
		assert.Len(t, tlsCfg.Certificates, c.wantCerts)
	}

	for _, c := range []struct {
		ctx          *types.SystemContext
		pathFragment string
	}{
		{&types.SystemContext{DockerDaemonCertPath: "/dev/null/this/does/not/exist"}, "dev/null/this/does/not/exist"},
	} {
		_, err := tlsConfig(c.ctx)
		assert.ErrorContains(t, err, c.pathFragment)
	}
}

func TestSpecifyPlainHTTPViaHostScheme(t *testing.T) {
	host := "http://127.0.0.1:2376"
	ctx := &types.SystemContext{
		DockerDaemonHost: host,
	}

	client, err := newDockerClient(ctx)

	assert.Nil(t, err, "There should be no error creating the Docker client")
	assert.NotNil(t, client, "A Docker client reference should have been returned")

	assert.Equal(t, host, client.DaemonHost())
	assert.NoError(t, client.Close())
}
