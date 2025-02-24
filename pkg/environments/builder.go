package environments

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/google/uuid"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
)

// -----------------------------------------------------------------------------
// Environment Builder
// -----------------------------------------------------------------------------

// Builder is a toolkit for building a new test Environment.
type Builder struct {
	Name string

	addons            clusters.Addons
	existingCluster   clusters.Cluster
	kubernetesVersion *semver.Version
}

// NewBuilder generates a new empty Builder for creating Environments.
func NewBuilder() *Builder {
	return &Builder{
		Name:   uuid.NewString(),
		addons: make(clusters.Addons),
	}
}

// WithName indicates a custom name to provide the testing environment
func (b *Builder) WithName(name string) *Builder {
	b.Name = name
	return b
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

// WithKubernetesVersion indicates which Kubernetes version to deploy clusters
// with, if the caller wants something other than the default.
func (b *Builder) WithKubernetesVersion(version semver.Version) *Builder {
	b.kubernetesVersion = &version
	return b
}

// Build is a blocking call to construct the configured Environment and it's
// underlying Kubernetes cluster. The amount of time that it blocks depends
// entirely on the underlying clusters.Cluster implementation that was requested.
func (b *Builder) Build(ctx context.Context) (Environment, error) {
	var cluster clusters.Cluster

	// determine if an existing cluster has been configured for deployment
	if b.existingCluster == nil {
		var err error
		builder := kind.NewBuilder().WithName(b.Name)
		if b.kubernetesVersion != nil {
			builder.WithClusterVersion(*b.kubernetesVersion)
		}
		cluster, err = builder.Build(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		if b.kubernetesVersion != nil {
			return nil, fmt.Errorf("can't provide kubernetes version when using an existing cluster")
		}
		cluster = b.existingCluster
	}

	// determine the addon dependencies of the cluster before building
	requiredAddons := make(map[string][]string)
	for _, addon := range b.addons {
		for _, dependency := range addon.Dependencies(ctx, cluster) {
			requiredAddons[string(dependency)] = append(requiredAddons[string(dependency)], string(addon.Name()))
		}
	}

	// verify addon dependency requirements have been met
	requiredAddonsThatAreMissing := make([]string, 0)
	for requiredAddon, neededBy := range requiredAddons {
		found := false
		for _, addon := range b.addons {
			if requiredAddon == string(addon.Name()) {
				found = true
				break
			}
		}
		if !found {
			requiredAddonsThatAreMissing = append(requiredAddonsThatAreMissing, fmt.Sprintf("%s (needed by %s)", requiredAddon, strings.Join(neededBy, ", ")))
		}
	}
	if len(requiredAddonsThatAreMissing) != 0 {
		return nil, fmt.Errorf("addon dependencies were not met, missing: %s", strings.Join(requiredAddonsThatAreMissing, ", "))
	}

	// run each addon deployment asynchronously and collect any errors that occur
	addonDeploymentErrorQueue := make(chan error, len(b.addons))
	for _, addon := range b.addons {
		addonCopy := addon
		go func() {
			if err := cluster.DeployAddon(ctx, addonCopy); err != nil {
				addonDeploymentErrorQueue <- fmt.Errorf("failed to deploy addon %s: %w", addonCopy.Name(), err)
			}
			addonDeploymentErrorQueue <- nil
		}()
	}

	// wait for all deployments to report, and gather up any errors
	collectedDeploymentErrorsCount := 0
	addonDeploymentErrors := make([]error, 0)
	for !(collectedDeploymentErrorsCount == len(b.addons)) {
		if err := <-addonDeploymentErrorQueue; err != nil {
			addonDeploymentErrors = append(addonDeploymentErrors, err)
		}
		collectedDeploymentErrorsCount++
	}

	// if any errors occurred during deployment, report them
	totalFailures := len(addonDeploymentErrors)
	switch totalFailures {
	case 0:
		return &environment{
			name:    b.Name,
			cluster: cluster,
		}, nil
	case 1:
		return nil, addonDeploymentErrors[0]
	default:
		errMsgs := make([]string, 0, totalFailures)
		for _, err := range addonDeploymentErrors {
			errMsgs = append(errMsgs, err.Error())
		}
		return nil, fmt.Errorf("%d addon deployments failed: %s", totalFailures, strings.Join(errMsgs, ", "))
	}
}
