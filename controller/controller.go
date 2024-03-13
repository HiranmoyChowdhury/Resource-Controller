package controller

import (
	"context"
	"fmt"
	"github.com/HiranmoyChowdhury/ResourceController/handler"
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

func (c *Controller) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()
	logger := klog.FromContext(ctx)

	logger.Info("Starting Ranchy controller")

	logger.Info("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(ctx.Done(), c.deploymentSynced, c.serviceSynced, c.ranchySynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logger.Info("Starting workers", "count", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	logger.Info("Started workers")
	<-ctx.Done()
	logger.Info("Shutting down workers")

	return nil
}

func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.workqueue.Get()
	logger := klog.FromContext(ctx)

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		if err := c.syncHandler(ctx, key); err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("we need to solve this problem '%s': %s, requeuing", key, err.Error())
		}
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

func (c *Controller) syncHandler(ctx context.Context, key string) error {
	logger := klog.LoggerWithValues(klog.FromContext(ctx), "resourceName", key)

	fmt.Println("syncHandler started")

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	ranchySt, err := c.ranchyLister.RanChies(namespace).Get(name)

	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("ranchy '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	deploymentName := c.GetDeploymentName(ranchySt)
	serviceName := c.GetServiceName(ranchySt)

	deployment, err := c.deploymentLister.Deployments(ranchySt.Namespace).Get(deploymentName)
	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(ranchySt.Namespace).Create(context.TODO(), newDeployment(ranchySt, deploymentName), metav1.CreateOptions{})
	}
	if err != nil {
		return err
	}

	service, err := c.serviceLister.Services(ranchySt.Namespace).Get(serviceName)

	if errors.IsNotFound(err) {
		service, err = c.kubeclientset.CoreV1().Services(ranchySt.Namespace).Create(context.TODO(), newService(ranchySt, serviceName), metav1.CreateOptions{})
	}
	if err != nil {
		return err
	}

	if (ranchySt.Spec.DeletionPolicy == "WipeOut") && !metav1.IsControlledBy(deployment, ranchySt) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		c.recorder.Event(ranchySt, corev1.EventTypeWarning, ErrResourceExists, msg)
	}
	if (ranchySt.Spec.DeletionPolicy == "WipeOut") && !metav1.IsControlledBy(service, ranchySt) {
		msg := fmt.Sprintf(MessageResourceExists, service.Name)
		c.recorder.Event(ranchySt, corev1.EventTypeWarning, ErrResourceExists, msg)
	}
	if (ranchySt.Spec.DeploymentSpec.Replicas != nil && *ranchySt.Spec.DeploymentSpec.Replicas != *deployment.Spec.Replicas) ||
		(ranchySt.Spec.DeploymentSpec.Image != "" && ranchySt.Spec.DeploymentSpec.Image != deployment.Spec.Template.Spec.Containers[0].Image) {
		logger.V(4).Info("Update deployment resource")
		deployment, err = c.kubeclientset.AppsV1().Deployments(ranchySt.Namespace).Update(context.TODO(), newDeployment(ranchySt, deploymentName), metav1.UpdateOptions{})
	}

	if (ranchySt.Spec.ServiceSpec.Port != nil && *ranchySt.Spec.ServiceSpec.Port != service.Spec.Ports[0].Port) ||
		(ranchySt.Spec.ServiceSpec.NodePort != nil && *ranchySt.Spec.ServiceSpec.NodePort != service.Spec.Ports[0].NodePort) ||
		(ranchySt.Spec.ServiceSpec.TargetPort != nil && *ranchySt.Spec.ServiceSpec.TargetPort != service.Spec.Ports[0].TargetPort.IntVal) {
		logger.V(4).Info("Update service resource")
		service, err = c.kubeclientset.CoreV1().Services(ranchySt.Namespace).Update(context.TODO(), newService(ranchySt, serviceName), metav1.UpdateOptions{})
	}
	err = c.updateRanchy(ranchySt, deployment, service)
	if err != nil {
		return err
	}

	c.recorder.Event(ranchySt, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateRanchy(ranchySt *rcsv1alpha1.RanChy, deployment *appsv1.Deployment, service *corev1.Service) error {

	ranchySt.Status.AvailableReplicas = &deployment.Status.AvailableReplicas

	_, err := c.rcsclientset.RcsV1alpha1().RanChies(ranchySt.Namespace).Update(context.TODO(), ranchySt, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	_, err = c.rcsclientset.RcsV1alpha1().RanChies(ranchySt.Namespace).UpdateStatus(context.TODO(), ranchySt, metav1.UpdateOptions{})
	return err
}

func (c *Controller) enqueueRanchy(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

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

}

func newDeployment(ranchySt *rcsv1alpha1.RanChy, name string) *appsv1.Deployment {

	labels := make(map[string]string)
	for k, v := range ranchySt.Spec.Labels {
		labels[k] = v
	}
	if len(labels) == 0 {
		labels = map[string]string{
			"owner": NextLabel(),
			"UID":   string(ranchySt.UID),
		}
	} else {
		labels["UID"] = string(ranchySt.UID)
	}
	deploymentName := name
	deploymentReplicaCount := ranchySt.Spec.DeploymentSpec.Replicas
	deploymentImage := ranchySt.Spec.DeploymentSpec.Image

	if deploymentImage == "" {
		deploymentImage = utils.DefaultImage
	}
	if &deploymentReplicaCount == nil {
		var replica int32 = utils.DefaultReplicaCount
		deploymentReplicaCount = &replica
	}

	objectMeta := metav1.ObjectMeta{}
	if deploymentName == "" {
		objectMeta.GenerateName = handler.ToLowerCase(ranchySt.Name)
	} else {
		objectMeta.Name = deploymentName
	}
	if ranchySt.Spec.DeletionPolicy == "WipeOut" {
		objectMeta.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(ranchySt, rcsv1alpha1.SchemeGroupVersion.WithKind("RanChy")),
		}
	}
	if ranchySt.ObjectMeta.Namespace != "" {
		objectMeta.Namespace = ranchySt.ObjectMeta.Namespace
	}

	objectMeta.Labels = labels

	containerPorts := []corev1.ContainerPort{}

	if ranchySt.Spec.ServiceSpec.TargetPort != nil {
		containerPorts[0].ContainerPort = *ranchySt.Spec.ServiceSpec.TargetPort
	}

	return &appsv1.Deployment{
		ObjectMeta: objectMeta,
		Spec: appsv1.DeploymentSpec{
			Replicas: deploymentReplicaCount,
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
							Name:    utils.ContainerName,
							Image:   deploymentImage,
							Command: ranchySt.Spec.DeploymentSpec.Commands,
							Ports:   containerPorts,
						},
					},
				},
			},
		},
	}
}

func newService(ranchySt *rcsv1alpha1.RanChy, name string) *corev1.Service {
	labels := make(map[string]string)
	for k, v := range ranchySt.Spec.Labels {
		labels[k] = v
	}
	if len(labels) == 0 {
		labels = map[string]string{
			"owner": NextLabel(),
			"UID":   string(ranchySt.UID),
		}
	} else {
		labels["UID"] = string(ranchySt.UID)
	}
	serviceName := name
	serviceType := ranchySt.Spec.ServiceSpec.ServiceType
	servicePort := ranchySt.Spec.ServiceSpec.Port

	if serviceType == "" {
		serviceType = utils.DefaultServiceType
	}
	if servicePort == nil {
		servicePort = GetPort()

	}
	if serviceType == "Headless" {
		serviceType = ""
	}

	perfectServiceType := corev1.ServiceType(serviceType)

	objectMeta := metav1.ObjectMeta{}
	if serviceName == "" {
		objectMeta.GenerateName = handler.ToLowerCase(ranchySt.Name)
	} else {
		serviceName = serviceName
		objectMeta.Name = serviceName
	}
	if ranchySt.Spec.DeletionPolicy == "WipeOut" {
		objectMeta.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(ranchySt, rcsv1alpha1.SchemeGroupVersion.WithKind("RanChy")),
		}
	}

	if ranchySt.ObjectMeta.Namespace != "" {
		objectMeta.Namespace = ranchySt.ObjectMeta.Namespace
	}
	objectMeta.Labels = labels

	ports := []corev1.ServicePort{
		{
			Port: *servicePort,
		},
	}
	if ranchySt.Spec.ServiceSpec.NodePort != nil {
		ports[0].NodePort = *ranchySt.Spec.ServiceSpec.NodePort
	}
	if ranchySt.Spec.ServiceSpec.TargetPort != nil {
		ports[0].TargetPort.IntVal = *ranchySt.Spec.ServiceSpec.TargetPort
	}

	return &corev1.Service{
		ObjectMeta: objectMeta,
		Spec: corev1.ServiceSpec{
			Ports:    ports,
			Selector: labels,
			Type:     perfectServiceType,
		},
	}
}
func (c *Controller) GetDeploymentName(r *rcsv1alpha1.RanChy) string {
	UID := string(r.UID)
	deploymentList, err := c.kubeclientset.AppsV1().Deployments(r.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "UID",
	})

	if err == nil {
		for _, deployment := range deploymentList.Items {
			if deployment.Labels["UID"] == UID {
				return deployment.Name
			}
		}
	}
	depName := r.Name
	if r.Spec.DeploymentSpec.Name != "" {
		depName += "-" + r.Spec.DeploymentSpec.Name
	}

	for i := 0; i != -1; i++ {
		name, err := c.findDeploymentNameValidation(r, depName, int32(i))
		if err == nil {
			return name
		}
	}

	return depName
}

func (c *Controller) findDeploymentNameValidation(r *rcsv1alpha1.RanChy, name string, cnt int32) (string, error) {
	_name := name + "-" + String(cnt)
	_, err := c.deploymentLister.Deployments(r.Namespace).Get(_name)
	if err != nil {
		return _name, nil
	}
	return "", fmt.Errorf("deployment Name already occupied")

}
func (c *Controller) GetServiceName(r *rcsv1alpha1.RanChy) string {
	UID := string(r.UID)
	serviceList, err := c.kubeclientset.CoreV1().Services(r.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "UID",
	})

	if err == nil {
		for _, service := range serviceList.Items {
			if service.Labels["UID"] == UID {
				return service.Name
			}
		}
	}
	svcName := r.Name
	if r.Spec.ServiceSpec.Name != "" {
		svcName += "-" + r.Spec.ServiceSpec.Name
	}

	for i := 0; i != 90000000; i++ {
		name, err := c.findServiceNameValidation(r, svcName, int32(i))
		if err == nil {
			return name
		}
	}

	return svcName
}

func (c *Controller) findServiceNameValidation(r *rcsv1alpha1.RanChy, name string, cnt int32) (string, error) {
	_name := name + "-" + String(cnt)
	_, err := c.serviceLister.Services(r.Namespace).Get(_name)
	if err != nil {
		return _name, nil
	}
	return "", fmt.Errorf("service Name already occupied")

}
