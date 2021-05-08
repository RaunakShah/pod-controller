package cmd

import (
	"context"
	"github.com/RaunakShah/custom-controller/pkg"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {

	}
	k8sclient, err := kubernetes.NewForConfig(config)
	pr, err := pkg.NewPodReconciler(k8sclient)
	pr.Run(context.TODO(), 10)

}
