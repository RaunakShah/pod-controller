package main

import (
	"context"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"time"
	"github.com/RaunakShah/custom-controller/pkg"
)

func main() {
	klog.Infof("1")
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Infof("Error")
	}
	klog.Infof("1")
	k8sclient, err := kubernetes.NewForConfig(config)
	klog.Infof("2")
	pr, err := pkg.NewPodReconciler(k8sclient)
	klog.Infof("3")
	pr.Run(context.TODO(), 10)
	klog.Infof("4")
	time.Sleep(5*time.Minute)
}
