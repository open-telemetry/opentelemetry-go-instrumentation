package main

import (
	"fmt"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/errors"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/instrumentors"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/opentelemetry"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/process"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	err := log.Init()
	if err != nil {
		fmt.Printf("could not init logger: %s\n", err)
		os.Exit(1)
	}

	log.Logger.V(0).Info("starting Go OpenTelemetry Agent ...")
	target := process.ParseTargetArgs()
	if err = target.Validate(); err != nil {
		log.Logger.Error(err, "invalid target args")
		return
	}

	processAnalyzer := process.NewAnalyzer()
	otelController, err := opentelemetry.NewController()
	if err != nil {
		log.Logger.Error(err, "unable to create OpenTelemetry controller")
		return
	}

	instManager, err := instrumentors.NewManager(otelController)
	if err != nil {
		log.Logger.Error(err, "error creating instrumetors manager")
		return
	}

	stopper := make(chan os.Signal, 1)
	signal.Notify(stopper, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stopper
		log.Logger.V(0).Info("Got SIGTERM, cleaning up..")
		processAnalyzer.Close()
		instManager.Close()
	}()

	pid, err := processAnalyzer.DiscoverProcessID(target)
	if err != nil {
		if err != errors.ErrInterrupted {
			log.Logger.Error(err, "error while discovering process id")
		}
		return
	}

	targetDetails, err := processAnalyzer.Analyze(pid, instManager.GetRelevantFuncs())
	if err != nil {
		log.Logger.Error(err, "error while analyzing target process")
		return
	}
	log.Logger.V(0).Info("target process analysis completed", "pid", targetDetails.PID,
		"go_version", targetDetails.GoVersion, "dependencies", targetDetails.Libraries,
		"total_functions_found", len(targetDetails.Functions))

	instManager.FilterUnusedInstrumentors(targetDetails)

	log.Logger.V(0).Info("invoking instrumentors")
	err = instManager.Run(targetDetails)
	if err != nil && err != errors.ErrInterrupted {
		log.Logger.Error(err, "error while running instrumentors")
	}
}
