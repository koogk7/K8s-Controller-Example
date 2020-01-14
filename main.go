package main

import (
	"K8s-Controller-Example/pkg/controller"
	"flag"
	"github.com/sirupsen/logrus"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	_ "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

func main() {
	var kubeconfig *string
	const namespace = "default"

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		logrus.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Fatal(err)
	}

	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (object runtime.Object, err error) {
				return clientset.AppsV1beta1().Deployments(namespace).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (w watch.Interface, err error) {
				return clientset.AppsV1beta1().Deployments(namespace).Watch(options)
			},
		},
		&appsv1beta1.Deployment{},
		1,
		cache.Indexers{},
	)

	controller := controller.NewResourceController(clientset, informer, "deploys")

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)

	select {}
}
