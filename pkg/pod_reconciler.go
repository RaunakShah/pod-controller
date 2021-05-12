package pkg

import (
	"context"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"time"
	"k8s.io/klog"
)

// PodReconciler is an interface to annotate pods with the current timestamp.
type PodReconciler interface {
	// Run starts the reconciler.
	Run(ctx context.Context, workers int)
}

// podReconciler implements the podReconciler interface.
// This implementation listens on pod add and update events.
// It adds the timestamp, as an annotation, to Pods if it
// doesnt already exist.
// If waitForAnnotation is set, this implementation only adds
// the timestamp to Pods that are annotated with "add-timestamp".
type podReconciler struct {
	k8sclient kubernetes.Interface
	podQueue workqueue.RateLimitingInterface
	podLister corev1lister.PodLister
	podSynced cache.InformerSynced
	waitForAnnotation bool
}

// NewPodReconciler initializes and returns an implementation of the PodReconciler interface.
// If the namespaceToWatch parameter is specified, it sets up the reconciler to listen only
// on the given namespace.
func NewPodReconciler(client kubernetes.Interface, namespaceToWatch string, waitForAnnotation bool) (PodReconciler, error) {
	klog.Infof("Initializing Pod annotation controller. Listening to Pods on namespace %s", namespaceToWatch)
	// Create new informer factory. If no namespace is specified, the informer is set up to listen on all namespaces.
	informerFactory := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithNamespace(namespaceToWatch))
	// Set up workqueue for Pod add and update events
	queue := workqueue.NewRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(time.Second, 5*time.Minute))
	podInformer := informerFactory.Core().V1().Pods()
	pr := &podReconciler{
		k8sclient: client,
		podQueue:  queue,
		podLister: podInformer.Lister(),
		podSynced: podInformer.Informer().HasSynced,
		waitForAnnotation: waitForAnnotation,
	}

	// Set up event handlers for Pod Add and Update events
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pr.podAdd,
		UpdateFunc: pr.podUpdate,
		DeleteFunc: nil,
	})

	// Channel that can be used to destroy the informer.
	stopCh := make(chan struct{})
	informerFactory.Start(stopCh)
	if !cache.WaitForCacheSync(stopCh, pr.podSynced) {
		return nil, nil
	}
	klog.Infof("Initialized Pod annotation controller.")
	return pr, nil
}
func (pr *podReconciler) podAdd(obj interface{}) {
	objKey, err := getPodKey(obj)
	if err != nil {
		klog.Errorf("failed to get pod key with error %v", err)
		return
	}
	klog.V(4).Infof("Adding Pod %s to queue", objKey)
	pr.podQueue.Add(objKey)
}


func (pr *podReconciler) podUpdate(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*v1.Pod)
	if !ok || oldPod == nil {
		return
	}

	newPod, ok := newObj.(*v1.Pod)
	if !ok || newPod == nil {
		return
	}

	pr.podAdd(newObj)
}


func (pr *podReconciler) Run(ctx context.Context, workers int) {
	defer pr.podQueue.ShutDown()

	klog.Infof("Setting up Pod Reconciler to run with %s threads", workers)
	stopCh := ctx.Done()

	for i := 0; i < workers; i++ {
		go wait.Until(pr.podReconcileWorker, 0, stopCh)
	}

	<-stopCh
}

// podReconcileWorker gets items from the workqueue and attempts to reconcile them.
// If reconciliation fails, it adds the item back to the workqueue.
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

// reconcile adds the current timestamp as an annotation to a Pod and logs the operation to stdout.
func (pr *podReconciler) reconcile (key string) error {
	klog.Infof("Reconciling Pod %s", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	pod, err := pr.podLister.Pods(namespace).Get(name)
	if err != nil {
		return err
	}
	podAnnotations := pod.GetAnnotations()
	if podAnnotations == nil {
		podAnnotations = make(map[string]string)
	}
	klog.Infof("Existing annotations on Pod %s/%s: %v", namespace, name, podAnnotations)
	if pr.waitForAnnotation {
		if _, ok := podAnnotations["add-timestamp"]; !ok {
			klog.Infof("Annotation add-timestamp doesnt exist, igonoring ...")
			return nil
		}
	}
	if _, ok := podAnnotations["timestamp"]; !ok {
		// Add timestamp to Pod
		podAnnotations["timestamp"] = time.Now().String()
		pod.SetAnnotations(podAnnotations)
		_, err = pr.k8sclient.CoreV1().Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		klog.Infof("Added timestamp %v to Pod %s/%s", podAnnotations["timestamp"], namespace, name)
	}
	return nil
}

func getPodKey(obj interface{}) (string, error) {
	if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
		obj = unknown.Obj
	}
	objKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	return objKey, err
}
