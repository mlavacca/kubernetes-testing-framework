package environments

import (
	"context"

	"github.com/google/uuid"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
)

// -----------------------------------------------------------------------------
// Environment Builder
// -----------------------------------------------------------------------------

// Builder is a toolkit for building a new test Environment.
type Builder struct {
	addons          clusters.Addons
	existingCluster clusters.Cluster
}

// NewBuilder generates a new empty Builder for creating Environments.
func NewBuilder() *Builder {
	return &Builder{addons: make(clusters.Addons)}
}

// WithAddons includes any provided Addon components in the cluster
// after the cluster is deployed.
func (b *Builder) WithAddons(addons ...clusters.Addon) *Builder {
	for _, addon := range addons {
		b.addons[addon.Name()] = addon
	}
	return b
}

// WithExistingCluster causes the resulting environment to re-use an existing
// clusters.Cluster instead of creating a new one.
func (b *Builder) WithExistingCluster(cluster clusters.Cluster) *Builder {
	b.existingCluster = cluster
	return b
}

// Build is a blocking call to construct the configured Environment and it's
// underlying Kubernetes cluster. The amount of time that it blocks depends
// entirely on the underlying clusters.Cluster implementation that was requested.
func (b *Builder) Build(ctx context.Context) (Environment, error) {
	var cluster clusters.Cluster

	if b.existingCluster == nil {
		var err error
		cluster, err = kind.New(ctx)
		if err != nil {
			return nil, err
		}
	}

	for _, addon := range b.addons {
		if err := cluster.DeployAddon(ctx, addon); err != nil {
			return nil, err
		}
	}

	return &environment{
		name:    uuid.NewString(),
		cluster: cluster,
	}, nil
}
