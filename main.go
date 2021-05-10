package main

import (
	"context"
	"os"

	"github.com/RaunakShah/custom-controller/pkg"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

func main() {
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
	// Initialize New Pod reconciler.
	pr, err := pkg.NewPodReconciler(k8sclient)
	if err != nil {
		klog.Errorf("failed to create new pod reconciler with error %v", err)
		os.Exit(1)
	}
	// Run the reconciler.
	pr.Run(context.TODO(), 10)

}
