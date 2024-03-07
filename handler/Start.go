package handler

import (
	"flag"
	"path/filepath"
	"sync"
	"time"

	"github.com/HiranmoyChowdhury/ResourceController/pkg/generated/clientset/versioned"
	"github.com/HiranmoyChowdhury/ResourceController/pkg/generated/informers/externalversions"
	"github.com/HiranmoyChowdhury/ResourceController/pkg/signals"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/HiranmoyChowdhury/ResourceController/pkg/generated/clientset/versioned"
	informers "github.com/HiranmoyChowdhury/ResourceController/pkg/generated/informers/externalversions"
)

func Start() {
	klog.InitFlags(nil)

	// set up signals so we handle the shutdown signal gracefully
	ctx := signals.SetupSignalHandler()
	logger := klog.FromContext(ctx)

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "path to the kubeconfig file")
	}
	flag.Parse()
	cfg, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic("Error building kubeconfig")
	}

	var wgkubeIF sync.WaitGroup
	var wgRcsIF sync.WaitGroup

	var kubeClient *kubernetes.Clientset
	var kubeInformerFactory kubeinformers.SharedInformerFactory
	go func() {
		kubeClient, err = kubernetes.NewForConfig(cfg)
		if err != nil {
			logger.Error(err, "Error building kubernetes clientset")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
			panic("Error building kubernetes clientset")
		}
		kubeInformerFactory = kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
		wgkubeIF.Done()
	}()

	var rcsClient *versioned.Clientset
	var rcsInformerFactory externalversions.SharedInformerFactory
	go func() {
		rcsClient, err = clientset.NewForConfig(cfg)
		if err != nil {
			logger.Error(err, "Error building kubernetes clientset")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
			panic("Error building kubernetes clientset")
		}
		rcsInformerFactory = informers.NewSharedInformerFactory(rcsClient, time.Second*30)
		wgRcsIF.Done()
	}()

	wgkubeIF.Wait()
	wgRcsIF.Wait()

	controller := NewController(ctx, kubeClient, rcsClient,
		kubeInformerFactory.Apps().V1().Deployments(),
		kubeInformerFactory.Core().V1().Services(),
		rcsInformerFactory.Rcs().V1alpha1().RanChies())

	kubeInformerFactory.Start(ctx.Done())
	rcsInformerFactory.Start(ctx.Done())

	/// Now it's time to run this contoller
	if err = controller.Run(ctx, 2); err != nil {
		logger.Error(err, "Error running controller")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
}
