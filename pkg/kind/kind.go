package kind

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// -----------------------------------------------------------------------------
// Public Consts & Vars
// -----------------------------------------------------------------------------

const (
	// DefaultKindDockerNetwork is the Docker network that a kind cluster uses by default.
	DefaultKindDockerNetwork = "kind"

	// KindContainerSuffix provides the string suffix that Kind names all cluster containers with.
	KindContainerSuffix = "-control-plane"

	// ProxyOnlyImage is the kind container image that should be used to deploy a Kind cluster that
	// is only running the Kong proxy, but no the ingress controller.
	// Note that images like this are built, maintained and documented here: https://github.com/kong/kind-images
	ProxyOnlyImage = "docker.pkg.github.com/kong/kind-images/proxy-only"
)

// -----------------------------------------------------------------------------
// Public Functions - Cluster Management
// -----------------------------------------------------------------------------

// CreateKindClusterWithKongProxy creates a new cluster using Kubernetes in Docker (KIND).
func CreateKindClusterWithKongProxy(name string) error {
	// TODO: for now using CLI and outputting to stdout/stderr
	// later we should switch to using the libs.
	cmd := exec.Command("kind", "create", "cluster", "--name", name, "--image", ProxyOnlyImage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DeleteKindCluster deletes an existing KIND cluster.
func DeleteKindCluster(name string) error {
	cmd := exec.Command("kind", "delete", "cluster", "--name", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// -----------------------------------------------------------------------------
// Public Functions - Helper
// -----------------------------------------------------------------------------

// GetKindContainerID produces the docker container ID for the given kind cluster by name.
func GetKindDockerContainerID(clusterName string) string {
	return fmt.Sprintf("%s%s", clusterName, KindContainerSuffix)
}

// ClientForKindCluster provides a *kubernetes.Clientset for a KIND cluster provided the cluster name.
func ClientForKindCluster(name string) (*kubernetes.Clientset, error) {
	kubeconfig := new(bytes.Buffer)
	cmd := exec.Command("kind", "get", "kubeconfig", "--name", name)
	cmd.Stdout = kubeconfig
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	clientCfg, err := clientcmd.NewClientConfigFromBytes(kubeconfig.Bytes())
	if err != nil {
		return nil, err
	}

	restCfg, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(restCfg)
}
