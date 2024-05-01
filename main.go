//go:build go1.18

/*
Copyright 2024 The Sage Group plc or its licensors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	err = cont.Watch(
		source.Kind(mgr.GetCache(), &corev1.Node{}),
		&handler.EnqueueRequestForObject{},
	)
	if err != nil {
		mainLog.Error(err, "Unable to watch nodes")
		os.Exit(1)
	}

	mainLog.Info("Starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		mainLog.Error(err, "Unable to start the manager")
		os.Exit(1)
	}
}
