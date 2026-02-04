package main

import (
	"context"
	"flag"
	"os"
	"strconv"
	"strings"

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

func logLevelFromEnv() (zapcore.Level, bool) {
	raw := strings.TrimSpace(os.Getenv("DPU_DAEMON_LOG_LEVEL"))
	if raw == "" {
		return 0, false
	}

	if parsed, err := zapcore.ParseLevel(strings.ToLower(raw)); err == nil {
		return parsed, true
	}

	verbosity, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	if verbosity <= 0 {
		return zapcore.InfoLevel, true
	}
	return zapcore.DebugLevel, true
}

func main() {
	opts := zap.Options{
		Development: true,
		Level:       zapcore.DebugLevel,
	}
	if level, ok := logLevelFromEnv(); ok {
		opts.Level = level
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
