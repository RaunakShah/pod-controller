package pkg

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"time"
)

type PodReconciler interface {
	Run(ctx context.Context, workers int)
}

type podReconciler struct {
	k8sclient kubernetes.Interface
	podQueue workqueue.RateLimitingInterface
	podLister corev1lister.PodLister
	podSynced cache.InformerSynced
}

func NewPodReconciler(client kubernetes.Interface) (PodReconciler, error) {
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	stopCh := make(chan struct{})
	queue := workqueue.NewRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(time.Second, 5*time.Minute))
	podInformer := informerFactory.Core().V1().Pods()
	pr := &podReconciler{
		k8sclient: client,
		podQueue:  queue,
		podLister: podInformer.Lister(),
		podSynced: podInformer.Informer().HasSynced,
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pr.podAdd,
//		UpdateFunc: pr.podUpdate,
		DeleteFunc: nil,
	})

	informerFactory.Start(stopCh)
	if !cache.WaitForCacheSync(stopCh, pr.podSynced) {
		return nil, nil
	}
	return pr, nil
}
func (pr *podReconciler) podAdd(obj interface{}) {
	objKey, err := getPodKey(obj)
	if err != nil {
		return
	}
	pr.podQueue.Add(objKey)
}

/*
func (pr *podReconciler) podUpdate(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*v1.Pod)
	if !ok || oldPod == nil {
		return
	}

	newPod, ok := newObj.(*v1.Pod)
	if !ok || newPod == nil {
		return
	}
	if oldPod.Status.Phase != v1.PodRunning && newPod.Status.Phase == v1.PodRunning {
		pr.podAdd(newObj)
	}
}
*/

func (pr *podReconciler) Run(ctx context.Context, workers int) {
	defer pr.podQueue.ShutDown()

	stopCh := ctx.Done()

	for i := 0; i < workers; i++ {
		go wait.Until(pr.podReconcileWorker, 0, stopCh)
	}

	<-stopCh
}

func (pr *podReconciler) podReconcileWorker() {
	key, quit := pr.podQueue.Get()
	if quit {
		return
	}
	defer pr.podQueue.Done(key)

	if err := pr.reconcile(key.(string)); err != nil {
		// Put PVC back to the queue so that we can retry later.
		pr.podQueue.AddRateLimited(key)
	} else {
		pr.podQueue.Forget(key)
	}
}

func (pr *podReconciler) reconcile (key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	pod, err := pr.podLister.Pods(namespace).Get(name)
	if err != nil {
		return err
	}
	podAnnotations := pod.GetAnnotations()
	if _, ok := podAnnotations["timestamp"]; !ok {
		// Add timestamp to Pod
		podAnnotations["timestamp"] = time.Now().String()
		pod.SetAnnotations(podAnnotations)
		_, err = pr.k8sclient.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
	}
	return nil
}

func getPodKey(obj interface{}) (string, error) {
	if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
		obj = unknown.Obj
	}
	objKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return "", err
	}
	return objKey, nil
}