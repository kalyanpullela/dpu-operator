package main

import (
	"context"
	"flag"
	"os"

	daemon "github.com/openshift/dpu-operator/internal/daemon"
	"github.com/openshift/dpu-operator/internal/platform"
	"go.uber.org/zap/zapcore"

	// Import vendor plugins to register them in the global registry
	_ "github.com/openshift/dpu-operator/pkg/plugin/intel"
	_ "github.com/openshift/dpu-operator/pkg/plugin/mangoboost"
	_ "github.com/openshift/dpu-operator/pkg/plugin/marvell"
	_ "github.com/openshift/dpu-operator/pkg/plugin/nvidia"
	_ "github.com/openshift/dpu-operator/pkg/plugin/xsight"

	"github.com/openshift/dpu-operator/internal/images"
	"github.com/openshift/dpu-operator/internal/utils"
	"github.com/spf13/afero"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	opts := zap.Options{
		Development: true,
		Level:       zapcore.DebugLevel,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	log := ctrl.Log.WithName("Daemon Init")
	log.Info("Daemon init")

	imageManager := images.NewEnvImageManager()

	nodeName := os.Getenv("K8S_NODE")

	platform := &platform.HardwarePlatform{}
	d := daemon.NewDaemon(afero.NewOsFs(), platform, ctrl.GetConfigOrDie(), imageManager, utils.NewPathManager("/"), nodeName)
	if err := d.PrepareAndServe(context.Background()); err != nil {
		log.Error(err, "Failed to run daemon")
		os.Exit(1)
	}
}
