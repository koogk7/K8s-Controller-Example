package controller

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"time"
)

/* Todo [issue] -
1. 오브젝트 상태 변화가 없는데도 계속 큐에 무엇이 들어간다.
2. 큐에 값이 들어와도, 출력이 되지 않는다.
	- 서로 다른 큐를 보고 있나? --> 그러면 초기에도 출력이 안되야 되지 않나?
*/

const (
	eventTypeAdd    = "ADD"
	eventTypeUpdate = "UPDATE"
	eventTypeDelete = "DELETE"
)

type Controller struct {
	logger   *logrus.Entry
	client   kubernetes.Interface
	informer cache.SharedIndexInformer
	queue    workqueue.RateLimitingInterface
}

type Event struct {
	key          string
	eventType    string
	namespace    string
	resourceType string
	content      interface{}
}

func NewEvent(key string, eventType string, namespace string, resourceType string, content interface{}) *Event {
	return &Event{key: key, eventType: eventType, namespace: namespace, resourceType: resourceType, content: content}
}

func NewResourceController(client kubernetes.Interface, informer cache.SharedIndexInformer, resourceType string) *Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pushQueueEvent(obj, eventTypeAdd, &queue)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pushQueueEvent(oldObj, eventTypeUpdate, &queue)
		},
		DeleteFunc: func(obj interface{}) {
			pushQueueEvent(obj, eventTypeDelete, &queue)
		},
	})

	return &Controller{
		logger:   logrus.WithField("pkg", "k8s-example-"+resourceType),
		client:   client,
		informer: informer,
		queue:    queue,
	}
}

func (receiver *Controller) Run(threadiness int, stopCh chan struct{}) {
	defer runtime.HandleCrash()
	defer receiver.queue.ShutDown()
	logrus.Info("Starting Deploy controller")

	go receiver.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, receiver.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(receiver.runWorker, time.Second*2, stopCh)
	}

	<-stopCh
}

func (receiver *Controller) runWorker() {
	for receiver.processNextItem() {
	}
}

func (receiver *Controller) processNextItem() bool {
	event, quit := receiver.queue.Get()
	if quit {
		logrus.Info("workqueue is empty")
		return false
	}
	defer receiver.queue.Done(event)
	err := receiver.syncToStdout(event.(Event))
	if err != nil {
		receiver.logger.Errorf("Error processsing %s ", err)
	} else {
		receiver.queue.Forget(event)
	}
	return true
}

func (receiver *Controller) syncToStdout(event Event) error {
	obj, _, err := receiver.informer.GetIndexer().GetByKey(event.key)
	if err != nil {
		return fmt.Errorf("Error fetching object with key %s from store : %v", event.key, err)
	}
	fmt.Printf("[%s] - %s\n", event.eventType, obj.(*appsv1beta1.Deployment).GetName())
	prettyPrint(event.content.(*appsv1beta1.Deployment).Status)

	return nil
}

func pushQueueEvent(object interface{}, eventType string, queue *workqueue.RateLimitingInterface) {
	var key string
	var err error

	if eventType == eventTypeDelete {
		key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(object)
	} else {
		key, err = cache.MetaNamespaceKeyFunc(object)
	}

	event := NewEvent(key, eventType, "default", "deploy", object)

	if err == nil {
		(*queue).Add(*event)
		logrus.Info("Complete to add: ")
		prettyPrint(event.content.(*appsv1beta1.Deployment).Status)
	}
}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	println(string(s))
	return string(s)
}
