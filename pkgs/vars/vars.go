package vars

import "os"

var (
	Namespace = "openshift-dpu-operator"

	DpuOperatorConfigName = "dpu-operator-config"

	DefaultHostNADName = "default-sriov-net"
)

// init allows overriding the namespace via environment variable.
// This is useful for running the operator outside the default namespace.
func init() {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		Namespace = ns
	}
}

const (
	MetricsServiceName               = "dpu-operator-controller-manager-metrics-service"
	DpuConfigVFCountAnnotationPrefix = "dpu.config.openshift.io/vf-count/"
)
