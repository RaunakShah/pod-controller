package main

import (
	"context"
	"k8s.io/api/core/v1"
	"log"
	"os"
	"flag"

	"github.com/kubernetes-csi/csi-lib-utils/leaderelection"

	"github.com/RaunakShah/custom-controller/pkg"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

var (
	namespaceToAddTimestamp = flag.String("add-timestamp-to-namespace", v1.NamespaceAll, "Adds timestamp only to Pods created in the specified namespace. Defaults to NamespaceAll")
	onlyRespondToAnnotation = flag.Bool("only-respond-to-annotation", false, "If set, reconciler only adds timestamp annotation to Pods that are annotated with add-timestamp")
	enableLeaderElection    = flag.Bool("leader-election", false, "Enable leader election.")
)
func main() {
	flag.Parse()
	klog.Infof("Creating Kubernetes client for Pod timestamp reconciler")
	// Create kubernetes client from in cluster config file.
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Errorf("failed to get config with error %v", err)
		os.Exit(1)
	}
	k8sclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Errorf("failed to create k8s client with error %v", err)
		os.Exit(1)
	}
	run := func(ctx context.Context){

		// Initialize New Pod reconciler.
		pr, err := pkg.NewPodReconciler(k8sclient, *namespaceToAddTimestamp, *onlyRespondToAnnotation)
		if err != nil {
			klog.Errorf("failed to create new pod reconciler with error %v", err)
			os.Exit(1)
		}
		// Run the reconciler.
		pr.Run(context.TODO(), 10)
	}
	if !*enableLeaderElection {
		run(context.TODO())
	} else {
		lockName := "pod-timestamp-annotation-reconciler"
		le := leaderelection.NewLeaderElection(k8sclient, lockName, run)

		if err := le.Run(); err != nil {
			log.Fatalf("Error initializing leader election: %v", err)
		}
	}
}
