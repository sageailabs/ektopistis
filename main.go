//go:build go1.18

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func main() {
	var drainOptions DrainOptions

	flag.StringVar(
		&drainOptions.DrainTaintName,
		"drain-taint-name",
		defaultDrainTaintName,
		"Name of the taing that will mark the nodes for draining")

	logOptions := zap.Options{}
	logOptions.BindFlags(flag.CommandLine)
	flag.Parse()
	log.SetLogger(zap.New(zap.UseFlagOptions(&logOptions)))
	mainLog := log.Log.WithName("main")

	mainLog.Info(fmt.Sprintf(
		"Starting with these flags:\n%s", strings.Join(os.Args[1:], "\n")))

	mainLog.Info("Setting up manager")
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		mainLog.Error(err, "Unable to set up controller manager")
		os.Exit(1)
	}

	mainLog.Info("Setting up controller")
	cont, err := controller.New("node-drainer", mgr, controller.Options{
		Reconciler: NewNodeDrainer(mgr.GetClient(), &drainOptions),
	})
	if err != nil {
		mainLog.Error(err, "Unable to set up node-drainer controller")
		os.Exit(1)
	}

	mainLog.Info("Setting up node watch")
	if err := cont.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForObject{}); err != nil {
		mainLog.Error(err, "Unable to watch nodes")
		os.Exit(1)
	}

	mainLog.Info("Starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		mainLog.Error(err, "Unable to start the manager")
		os.Exit(1)
	}
}
