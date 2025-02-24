package kind

import (
	"context"
	"fmt"
	"sync"

	"github.com/blang/semver/v4"
	"github.com/google/uuid"

	"github.com/kong/kubernetes-testing-framework/internal/utils"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// Builder generates clusters.Cluster objects backed by Kind given
// provided configuration options.
type Builder struct {
	Name string

	addons         clusters.Addons
	clusterVersion *semver.Version
	configPath     *string
}

// NewBuilder provides a new *Builder object.
func NewBuilder() *Builder {
	return &Builder{
		Name:   uuid.NewString(),
		addons: make(clusters.Addons),
	}
}

// WithName indicates a custom name to use for the cluster.
func (b *Builder) WithName(name string) *Builder {
	b.Name = name
	return b
}

// WithClusterVersion configures the Kubernetes cluster version for the Builder
// to use when building the Kind cluster.
func (b *Builder) WithClusterVersion(version semver.Version) *Builder {
	b.clusterVersion = &version
	return b
}

// WithConfig sets a filename containing a KIND config
// See: https://kind.sigs.k8s.io/docs/user/configuration/
func (b *Builder) WithConfig(filename string) *Builder {
	b.configPath = &filename
	return b
}

// Build creates and configures clients for a Kind-based Kubernetes clusters.Cluster.
func (b *Builder) Build(ctx context.Context) (*Cluster, error) {
	deployArgs := make([]string, 0)
	if b.clusterVersion != nil {
		deployArgs = append(deployArgs, "--image", "kindest/node:v"+b.clusterVersion.String())
	}

	if b.configPath != nil {
		deployArgs = append(deployArgs, "--config", *b.configPath)
	}

	if err := createCluster(ctx, b.Name, deployArgs...); err != nil {
		return nil, fmt.Errorf("failed to create cluster %s: %w", b.Name, err)
	}

	cfg, kc, err := clientForCluster(b.Name)
	if err != nil {
		return nil, err
	}

	cluster := &Cluster{
		name:       b.Name,
		client:     kc,
		cfg:        cfg,
		addons:     make(clusters.Addons),
		deployArgs: deployArgs,
		l:          &sync.RWMutex{},
	}

	if err := utils.ClusterInitHooks(ctx, cluster); err != nil {
		if cleanupErr := cluster.Cleanup(ctx); cleanupErr != nil {
			return nil, fmt.Errorf("multiple errors occurred BUILD_ERROR=(%s) CLEANUP_ERROR=(%s)", err, cleanupErr)
		}
		return nil, err
	}

	return cluster, err
}
