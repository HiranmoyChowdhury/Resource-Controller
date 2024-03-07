package handler

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"

	"github.com/HiranmoyChowdhury/ResourceController/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	rcsv1alpha1 "github.com/HiranmoyChowdhury/ResourceController/pkg/apis/rcs/v1alpha1"
	clientset "github.com/HiranmoyChowdhury/ResourceController/pkg/generated/clientset/versioned"
	rcsscheme "github.com/HiranmoyChowdhury/ResourceController/pkg/generated/clientset/versioned/scheme"
	informers "github.com/HiranmoyChowdhury/ResourceController/pkg/generated/informers/externalversions/rcs/v1alpha1"
	listers "github.com/HiranmoyChowdhury/ResourceController/pkg/generated/listers/rcs/v1alpha1"
)

const controllerAgentName = "RanChy-controller"

const (
	SuccessSynced         = "Synced"
	ErrResourceExists     = "ErrResourceExists"
	MessageResourceExists = "Resource %q already exists and is not managed by RanChy"
	MessageResourceSynced = "RanChy synced successfully"
)

type Controller struct {
	kubeclientset kubernetes.Interface
	rcsclientset  clientset.Interface

	deploymentLister appslisters.DeploymentLister
	deploymentSynced cache.InformerSynced
	serviceLister    corelisters.ServiceLister
	serviceSynced    cache.InformerSynced
	ranchyLister     listers.RanChyLister
	ranchySynced     cache.InformerSynced

	workqueue workqueue.RateLimitingInterface

	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	ctx context.Context,
	kubeclientset kubernetes.Interface,
	rcsclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	serviceInformer coreinformer.ServiceInformer,
	ranchyInformer informers.RanChyInformer) *Controller {
	logger := klog.FromContext(ctx)

	utilruntime.Must(rcsscheme.AddToScheme(scheme.Scheme))
	logger.V(4).Info("Creating event broadcaster")

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	ratelimiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 1000*time.Second),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	controller := &Controller{
		kubeclientset:    kubeclientset,
		rcsclientset:     rcsclientset,
		deploymentLister: deploymentInformer.Lister(),
		deploymentSynced: deploymentInformer.Informer().HasSynced,
		serviceLister:    serviceInformer.Lister(),
		serviceSynced:    serviceInformer.Informer().HasSynced,
		ranchyLister:     ranchyInformer.Lister(),
		ranchySynced:     ranchyInformer.Informer().HasSynced,
		workqueue:        workqueue.NewRateLimitingQueue(ratelimiter),
		recorder:         recorder,
	}

	logger.Info("Setting up event handlers")

	ranchyInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueRanchy,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueRanchy(new)
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueueRanchy(obj)
		},
	})

	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newServ := new.(*corev1.Service)
			oldServ := old.(*corev1.Service)
			if newServ.ResourceVersion == oldServ.ResourceVersion {
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()
	logger := klog.FromContext(ctx)

	// Start the informer factories to begin populating the informer caches
	logger.Info("Starting Ranchy controller")

	// Wait for the caches to be synced before starting workers
	logger.Info("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(ctx.Done(), c.deploymentSynced, c.serviceSynced, c.ranchySynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logger.Info("Starting workers", "count", workers)
	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	logger.Info("Started workers")
	<-ctx.Done()
	logger.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.workqueue.Get()
	logger := klog.FromContext(ctx)

	if shutdown {
		return false
	}

	// We wrap this block in a func, so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// RanChy resource to be synced.
		if err := c.syncHandler(ctx, key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		logger.Info("Successfully synced", "resourceName", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Foo resource
// with the current status of the resource.
func (c *Controller) syncHandler(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	logger := klog.LoggerWithValues(klog.FromContext(ctx), "resourceName", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Foo resource with this namespace/name
	ranchySt, err := c.ranchyLister.RanChies(namespace).Get(name)
	if err != nil {
		// The Foo resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("ranchy '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	deploymentName := ranchySt.Spec.DeploymentName
	serviceName := ranchySt.Spec.ServiceName
	if deploymentName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
		return nil
	}
	if serviceName == "" {
		utilruntime.HandleError(fmt.Errorf("%s: service name must be specified", key))
		return nil
	}

	// Get the deployment with the name specified in Foo.spec
	deployment, err := c.deploymentLister.Deployments(ranchySt.Namespace).Get(deploymentName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(ranchySt.Namespace).Create(context.TODO(), newDeployment(ranchySt), metav1.CreateOptions{})
		c.updateForDeployment(ranchySt, deployment)
	}
	if err != nil {
		return err
	}

	service, err := c.serviceLister.Services(ranchySt.Namespace).Get(serviceName)

	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		service, err = c.kubeclientset.CoreV1().Services(ranchySt.Namespace).Create(context.TODO(), newService(ranchySt), metav1.CreateOptions{})
	}
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Deployment is not controlled by this Foo resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(deployment, ranchySt) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		c.recorder.Event(ranchySt, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf("%s", msg)
	}
	if !metav1.IsControlledBy(service, ranchySt) {
		msg := fmt.Sprintf(MessageResourceExists, service.Name)
		c.recorder.Event(ranchySt, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf("%s", msg)
	}

	// If this number of the replicas on the Foo resource is specified, and the
	// number does not equal the current desired replicas on the Deployment, we
	// should update the Deployment resource.
	if (ranchySt.Spec.Replicas != nil && *ranchySt.Spec.Replicas != *deployment.Spec.Replicas) ||
		(ranchySt.Spec.DeploymentName != "" && ranchySt.Spec.DeploymentName != deployment.Name) ||
		(ranchySt.Spec.DeploymentImage != "" && ranchySt.Spec.DeploymentImage != deployment.Spec.Template.Spec.Containers[0].Image) {
		logger.V(4).Info("Update deployment resource")
		deployment, err = c.kubeclientset.AppsV1().Deployments(ranchySt.Namespace).Update(context.TODO(), newDeployment(ranchySt), metav1.UpdateOptions{})
		c.updateForDeployment(ranchySt, deployment)
	}

	if err != nil {
		return err
	}

	if (ranchySt.Spec.ServiceName != "" && ranchySt.Spec.ServiceName != service.Name) ||
		(ranchySt.Spec.ServicePort != nil && *ranchySt.Spec.ServicePort != service.Spec.Ports[0].Port) ||
		(ranchySt.Spec.ServicePort != nil && *ranchySt.Spec.ServicePort != service.Spec.Ports[0].NodePort) ||
		(ranchySt.Spec.ServicePort != nil && *ranchySt.Spec.ServicePort != service.Spec.Ports[0].TargetPort.IntVal) {
		logger.V(4).Info("Update service resource")
		service, err = c.kubeclientset.CoreV1().Services(ranchySt.Namespace).Update(context.TODO(), newService(ranchySt), metav1.UpdateOptions{})

	}
	if err != nil {
		return err
	}

	err = c.updateRanchyStatus(ranchySt, deployment, service)
	if err != nil {
		return err
	}

	c.recorder.Event(ranchySt, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateRanchyStatus(ranchySt *rcsv1alpha1.RanChy, deployment *appsv1.Deployment, service *corev1.Service) error {
	ranchyCopy := ranchySt.DeepCopy()
	ranchyCopy.Status.AvailableReplicas = &deployment.Status.AvailableReplicas

	_, err := c.rcsclientset.RcsV1alpha1().RanChies(ranchySt.Namespace).UpdateStatus(context.TODO(), ranchyCopy, metav1.UpdateOptions{})
	return err
}

// enqueueFoo takes a Ranchy resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Ranchy.
func (c *Controller) enqueueRanchy(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Ranchy resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Foo resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	logger := klog.FromContext(context.Background())
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		logger.V(4).Info("Recovered deleted object", "resourceName", object.GetName())
	}
	logger.V(4).Info("Processing object", "object", klog.KObj(object))
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Ranchy, we should not do anything more
		// with it.
		if ownerRef.Kind != "RanChy" {
			return
		}

		ranchy, err := c.ranchyLister.RanChies(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			logger.V(4).Info("Ignore orphaned object", "object", klog.KObj(object), "foo", ownerRef.Name)
			return
		}

		c.enqueueRanchy(ranchy)
		return
	}
}

func newDeployment(ranchySt *rcsv1alpha1.RanChy) *appsv1.Deployment {
	labels := ranchySt.Spec.Labels
	deploymentName := ranchySt.Spec.DeploymentName
	deploymentReplicaCount := *ranchySt.Spec.Replicas
	deploymentImage := ranchySt.Spec.DeploymentImage

	if deploymentImage == "" {
		deploymentImage = utils.DefaultImage
	}

	if &deploymentReplicaCount == nil || deploymentReplicaCount == 0 {
		deploymentReplicaCount = utils.DefaultReplicaCount
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ranchySt, rcsv1alpha1.SchemeGroupVersion.WithKind("RanChy")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &deploymentReplicaCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  utils.ContainerName,
							Image: deploymentImage,
						},
					},
				},
			},
		},
	}
}

func newService(ranchySt *rcsv1alpha1.RanChy) *corev1.Service {
	labels := ranchySt.Spec.Labels
	serviceName := ranchySt.Spec.ServiceName
	serviceType := ranchySt.Spec.ServiceType
	servicePort := ranchySt.Spec.ServicePort

	if serviceType == "" {
		serviceType = utils.DefaultServiceType
	}
	if servicePort == nil {
		*servicePort = utils.DefaultServicePort
	}
	if serviceType == "Headless" {
		serviceType = ""
	}
	perfectServiceType := corev1.ServiceType(serviceType)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ranchySt, rcsv1alpha1.SchemeGroupVersion.WithKind("RanChy")),
			},
			Labels: labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:     *servicePort,
					NodePort: *servicePort,
					Protocol: "TCP",
				},
			},
			Selector: labels,
			Type:     perfectServiceType,
		},
	}
}

func (c *Controller) updateForDeployment(ranchySt *rcsv1alpha1.RanChy, deployment *appsv1.Deployment) {
	deploymentName := deployment.GenerateName
	ranchySt.Spec.DeploymentName = deploymentName
	deployment.Name = deploymentName
	deployment, _ = c.kubeclientset.AppsV1().Deployments(ranchySt.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	c.enqueueRanchy(ranchySt)
}
func (c *Controller) updateForService(ranchySt *rcsv1alpha1.RanChy, service *corev1.Service) {
	serviceName := service.GenerateName
	ranchySt.Spec.ServiceName = serviceName
	service.Name = serviceName
	service, _ = c.kubeclientset.CoreV1().Services(ranchySt.Namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
	c.enqueueRanchy(ranchySt)
}
