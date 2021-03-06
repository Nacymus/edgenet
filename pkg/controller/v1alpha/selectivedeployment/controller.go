/*
Copyright 2020 Sorbonne Université

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package selectivedeployment

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"

	apps_v1alpha "github.com/EdgeNet-project/edgenet/pkg/apis/apps/v1alpha"
	"github.com/EdgeNet-project/edgenet/pkg/generated/clientset/versioned"
	appsinformer_v1alpha "github.com/EdgeNet-project/edgenet/pkg/generated/informers/externalversions/apps/v1alpha"
	"github.com/EdgeNet-project/edgenet/pkg/node"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// The main structure of controller
type controller struct {
	logger              *log.Entry
	queue               workqueue.RateLimitingInterface
	informer            cache.SharedIndexInformer
	nodeInformer        cache.SharedIndexInformer
	deploymentInformer  cache.SharedIndexInformer
	daemonSetInformer   cache.SharedIndexInformer
	statefulSetInformer cache.SharedIndexInformer
	jobInformer         cache.SharedIndexInformer
	cronJobInformer     cache.SharedIndexInformer
	handler             HandlerInterface
	wg                  map[string]*sync.WaitGroup
}

// The main structure of informerevent
type informerevent struct {
	key      string
	function string
}

// Definitions of the state of the selectivedeployment resource (failure, partial, success)
const failure = "Failure"
const partial = "Running Partially"
const success = "Running"
const noSchedule = "NoSchedule"
const create = "create"
const update = "update"
const delete = "delete"
const trueStr = "True"
const falseStr = "False"
const unknownStr = "Unknown"

// Dictionary of status messages
var statusDict = map[string]string{
	"sd-success":                   "The selective deployment smoothly created the workload(s)",
	"deployment-creation-failure":  "Deployment %s could not be created",
	"deployment-in-use":            "Deployment %s is already under the control of another selective deployment",
	"daemonset-creation-failure":   "DaemonSet %s could not be created",
	"daemonset-in-use":             "DaemonSet %s is already under the control of another selective deployment",
	"statefulset-creation-failure": "StatefulSet %s could not be created",
	"statefulset-in-use":           "StatefulSet %s is already under the control of another selective deployment",
	"job-creation-failure":         "Job %s could not be created",
	"job-in-use":                   "Job %s is already under the control of another selective deployment",
	"cronjob-creation-failure":     "CronJob %s could not be created",
	"cronjob-in-use":               "CronJob %s is already under the control of another selective deployment",
	"nodes-fewer":                  "Fewer nodes issue, %d node(s) found instead of %d for %s%s",
	"GeoJSON-err":                  "%s%s has a GeoJSON format error",
}

// Start function is entry point of the controller
func Start(kubernetes kubernetes.Interface, edgenet versioned.Interface) {
	var err error
	clientset := kubernetes
	edgenetClientset := edgenet

	wg := make(map[string]*sync.WaitGroup)
	sdHandler := &SDHandler{}
	// Create the selectivedeployment informer which was generated by the code generator to list and watch selectivedeployment resources
	informer := appsinformer_v1alpha.NewSelectiveDeploymentInformer(
		edgenetClientset,
		metav1.NamespaceAll,
		0,
		cache.Indexers{},
	)
	// Create a work queue which contains a key of the resource to be handled by the handler
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	var event informerevent
	// Event handlers deal with events of resources. In here, we take into consideration of adding and updating selectivedeployments
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// Put the resource object into a key
			event.key, err = cache.MetaNamespaceKeyFunc(obj)
			event.function = create
			log.Infof("Add selectivedeployment: %s", event.key)
			if err == nil {
				// Add the key to the queue
				queue.Add(event)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if reflect.DeepEqual(oldObj.(*apps_v1alpha.SelectiveDeployment).Status, newObj.(*apps_v1alpha.SelectiveDeployment).Status) {
				event.key, err = cache.MetaNamespaceKeyFunc(newObj)
				event.function = update
				log.Infof("Update selectivedeployment: %s", event.key)
				if err == nil {
					queue.Add(event)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			// DeletionHandlingMetaNamsespaceKeyFunc helps to check the existence of the object while it is still contained in the index.
			// Put the resource object into a key
			event.key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			event.function = delete
			log.Infof("Delete selectivedeployment: %s", event.key)
			if err == nil {
				queue.Add(event)
			}
		},
	})

	// The selectivedeployment resources are reconfigured according to node events in this section
	nodeInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			// The main purpose of listing is to attach geo labels to whole nodes at the beginning
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return clientset.CoreV1().Nodes().List(context.TODO(), options)
			},
			// This function watches all changes/updates of nodes
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return clientset.CoreV1().Nodes().Watch(context.TODO(), options)
			},
		},
		&corev1.Node{},
		0,
		cache.Indexers{},
	)
	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			nodeObj := obj.(*corev1.Node)
			for _, conditionRow := range nodeObj.Status.Conditions {
				if conditionType := conditionRow.Type; conditionType == "Ready" {
					if conditionRow.Status == trueStr {
						key, err := cache.MetaNamespaceKeyFunc(obj)
						if err != nil {
							log.Println(err.Error())
							panic(err.Error())
						}
						sdRaw, _ := edgenetClientset.AppsV1alpha().SelectiveDeployments("").List(context.TODO(), metav1.ListOptions{})
						for _, sdRow := range sdRaw.Items {
							if sdRow.Spec.Recovery {
								if sdRow.Status.State == partial || sdRow.Status.State == failure {
								selectorLoop:
									for _, selectorDet := range sdRow.Spec.Selector {
										fewerNodes := false
										for _, message := range sdRow.Status.Message {
											if strings.Contains(message, "Fewer nodes issue") {
												fewerNodes = true
											}
										}
										if selectorDet.Quantity == 0 || (selectorDet.Quantity != 0 && fewerNodes) {
											event.key, err = cache.MetaNamespaceKeyFunc(sdRow.DeepCopyObject())
											event.function = update
											log.Infof("SD node added: %s, recovery started for: %s", key, event.key)
											if err == nil {
												queue.Add(event)
											}
											break selectorLoop
										}
									}
								}
							}
						}
					}
				}
			}
		},
		UpdateFunc: func(old, new interface{}) {
			oldObj := old.(*corev1.Node)
			newObj := new.(*corev1.Node)
			oldReady := node.GetConditionReadyStatus(oldObj)
			newReady := node.GetConditionReadyStatus(newObj)
			if (oldReady == falseStr && newReady == trueStr) ||
				(oldReady == unknownStr && newReady == trueStr) ||
				(oldObj.Spec.Unschedulable == true && newObj.Spec.Unschedulable == false) {
				key, err := cache.MetaNamespaceKeyFunc(newObj)
				if err != nil {
					log.Println(err.Error())
					panic(err.Error())
				}
				sdRaw, _ := edgenetClientset.AppsV1alpha().SelectiveDeployments("").List(context.TODO(), metav1.ListOptions{})
				for _, sdRow := range sdRaw.Items {
					if sdRow.Spec.Recovery {
						if sdRow.Status.State == partial || sdRow.Status.State == failure {
						selectorLoop:
							for _, selectorDet := range sdRow.Spec.Selector {
								fewerNodes := false
								for _, message := range sdRow.Status.Message {
									if strings.Contains(message, "Fewer nodes issue") {
										fewerNodes = true
									}
								}
								if selectorDet.Quantity == 0 || (selectorDet.Quantity != 0 && fewerNodes) {
									event.key, err = cache.MetaNamespaceKeyFunc(sdRow.DeepCopyObject())
									event.function = update
									log.Infof("SD node updated: %s, recovery started for: %s", key, event.key)
									if err == nil {
										queue.Add(event)
									}
									break selectorLoop
								}
							}
						}
					}
				}
			} else if updated := node.CompareIPAddresses(oldObj, newObj); (oldReady == trueStr && newReady == falseStr) ||
				(oldReady == trueStr && newReady == unknownStr) ||
				(oldObj.Spec.Unschedulable == false && newObj.Spec.Unschedulable == true) ||
				(newObj.Spec.Unschedulable == false && newReady == trueStr && updated) {
				key, err := cache.MetaNamespaceKeyFunc(newObj.DeepCopyObject())
				if err != nil {
					log.Println(err.Error())
					panic(err.Error())
				}
				ownerList, status := sdHandler.getByNode(newObj.GetName())
				if status {
					for _, ownerDet := range ownerList {
						sdObj, err := edgenetClientset.AppsV1alpha().SelectiveDeployments(ownerDet[0]).Get(context.TODO(), ownerDet[1], metav1.GetOptions{})
						if err != nil {
							continue
						}
						if sdObj.Spec.Recovery {
							event.key, err = cache.MetaNamespaceKeyFunc(sdObj.DeepCopyObject())
							event.function = update
							log.Infof("SD node updated: %s, recovery started for: %s", key, event.key)
							if err == nil {
								queue.Add(event)
							}
						}
					}
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			nodeObj := obj.(*corev1.Node)
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				log.Println(err.Error())
				panic(err.Error())
			}
			ownerList, status := sdHandler.getByNode(nodeObj.GetName())
			if status {
				for _, ownerDet := range ownerList {
					sdObj, err := edgenetClientset.AppsV1alpha().SelectiveDeployments(ownerDet[0]).Get(context.TODO(), ownerDet[1], metav1.GetOptions{})
					if err != nil {
						log.Println(err.Error())
						continue
					}
					if sdObj.Spec.Recovery {
						event.key, err = cache.MetaNamespaceKeyFunc(sdObj.DeepCopyObject())
						event.function = update
						log.Infof("SD node deleted: %s, recovery started for: %s", key, event.key)
						if err == nil {
							queue.Add(event)
						}
					}
				}
			}
		},
	})

	// The selectivedeployment resources are reconfigured according to workload events in this section
	addToQueue := func(ownerSD *apps_v1alpha.SelectiveDeployment, key string, ctlType string) {
		event.key, err = cache.MetaNamespaceKeyFunc(ownerSD.DeepCopyObject())
		event.function = update
		log.Infof("SD %s added: %s, recovery started for: %s", ctlType, key, event.key)
		if err == nil {
			queue.Add(event)
		}
	}

	workloadAddFunc := func(obj interface{}) {
		switch workloadObj := obj.(type) {
		case *appsv1.Deployment:
			var sdName string
			underControl := false
			ownerReferences := workloadObj.GetOwnerReferences()
			for _, reference := range ownerReferences {
				if reference.Kind == "SelectiveDeployment" {
					underControl = true
					sdName = reference.Name
				}
			}
			if !underControl {
				ownerSD, err := edgenetClientset.AppsV1alpha().SelectiveDeployments(workloadObj.GetNamespace()).Get(context.TODO(), sdName, metav1.GetOptions{})
				if err == nil {
					key, _ := cache.MetaNamespaceKeyFunc(workloadObj)
					addToQueue(ownerSD, key, "Deployment")
				}
			}
		case *appsv1.DaemonSet:
			var sdName string
			underControl := false
			ownerReferences := workloadObj.GetOwnerReferences()
			for _, reference := range ownerReferences {
				if reference.Kind == "SelectiveDeployment" {
					underControl = true
					sdName = reference.Name
				}
			}
			if !underControl {
				ownerSD, err := edgenetClientset.AppsV1alpha().SelectiveDeployments(workloadObj.GetNamespace()).Get(context.TODO(), sdName, metav1.GetOptions{})
				if err == nil {
					key, _ := cache.MetaNamespaceKeyFunc(workloadObj)
					addToQueue(ownerSD, key, "DaemonSet")
				}
			}
		case *appsv1.StatefulSet:
			var sdName string
			underControl := false
			ownerReferences := workloadObj.GetOwnerReferences()
			for _, reference := range ownerReferences {
				if reference.Kind == "SelectiveDeployment" {
					underControl = true
					sdName = reference.Name
				}
			}
			if !underControl {
				ownerSD, err := edgenetClientset.AppsV1alpha().SelectiveDeployments(workloadObj.GetNamespace()).Get(context.TODO(), sdName, metav1.GetOptions{})
				if err == nil {
					key, _ := cache.MetaNamespaceKeyFunc(workloadObj)
					addToQueue(ownerSD, key, "StatefulSet")
				}
			}
		case *batchv1.Job:
			var sdName string
			underControl := false
			ownerReferences := workloadObj.GetOwnerReferences()
			for _, reference := range ownerReferences {
				if reference.Kind == "SelectiveDeployment" {
					underControl = true
					sdName = reference.Name
				}
			}
			if !underControl {
				ownerSD, err := edgenetClientset.AppsV1alpha().SelectiveDeployments(workloadObj.GetNamespace()).Get(context.TODO(), sdName, metav1.GetOptions{})
				if err == nil {
					key, _ := cache.MetaNamespaceKeyFunc(workloadObj)
					addToQueue(ownerSD, key, "Job")
				}
			}
		case *batchv1beta.CronJob:
			var sdName string
			underControl := false
			ownerReferences := workloadObj.GetOwnerReferences()
			for _, reference := range ownerReferences {
				if reference.Kind == "SelectiveDeployment" {
					underControl = true
					sdName = reference.Name
				}
			}
			if !underControl {
				ownerSD, err := edgenetClientset.AppsV1alpha().SelectiveDeployments(workloadObj.GetNamespace()).Get(context.TODO(), sdName, metav1.GetOptions{})
				if err == nil {
					key, _ := cache.MetaNamespaceKeyFunc(workloadObj)
					addToQueue(ownerSD, key, "CronJob")
				}
			}
		}
	}
	workloadDeleteFunc := func(obj interface{}) {
		switch workloadObj := obj.(type) {
		case *appsv1.Deployment:
			ownerReferences := workloadObj.GetOwnerReferences()
			for _, reference := range ownerReferences {
				if reference.Kind == "SelectiveDeployment" {
					ownerSD, err := edgenetClientset.AppsV1alpha().SelectiveDeployments(workloadObj.GetNamespace()).Get(context.TODO(), reference.Name, metav1.GetOptions{})
					if err == nil {
						key, _ := cache.MetaNamespaceKeyFunc(workloadObj)
						addToQueue(ownerSD, key, "Deployment")
					}
				}
			}
		case *appsv1.DaemonSet:
			ownerReferences := workloadObj.GetOwnerReferences()
			for _, reference := range ownerReferences {
				if reference.Kind == "SelectiveDeployment" {
					ownerSD, err := edgenetClientset.AppsV1alpha().SelectiveDeployments(workloadObj.GetNamespace()).Get(context.TODO(), reference.Name, metav1.GetOptions{})
					if err == nil {
						key, _ := cache.MetaNamespaceKeyFunc(workloadObj)
						addToQueue(ownerSD, key, "DaemonSet")
					}
				}
			}
		case *appsv1.StatefulSet:
			ownerReferences := workloadObj.GetOwnerReferences()
			for _, reference := range ownerReferences {
				if reference.Kind == "SelectiveDeployment" {
					ownerSD, err := edgenetClientset.AppsV1alpha().SelectiveDeployments(workloadObj.GetNamespace()).Get(context.TODO(), reference.Name, metav1.GetOptions{})
					if err == nil {
						key, _ := cache.MetaNamespaceKeyFunc(workloadObj)
						addToQueue(ownerSD, key, "StatefulSet")
					}
				}
			}
		case *batchv1.Job:
			ownerReferences := workloadObj.GetOwnerReferences()
			for _, reference := range ownerReferences {
				if reference.Kind == "SelectiveDeployment" {
					ownerSD, err := edgenetClientset.AppsV1alpha().SelectiveDeployments(workloadObj.GetNamespace()).Get(context.TODO(), reference.Name, metav1.GetOptions{})
					if err == nil {
						key, _ := cache.MetaNamespaceKeyFunc(workloadObj)
						addToQueue(ownerSD, key, "Job")
					}
				}
			}
		case *batchv1beta.CronJob:
			ownerReferences := workloadObj.GetOwnerReferences()
			for _, reference := range ownerReferences {
				if reference.Kind == "SelectiveDeployment" {
					ownerSD, err := edgenetClientset.AppsV1alpha().SelectiveDeployments(workloadObj.GetNamespace()).Get(context.TODO(), reference.Name, metav1.GetOptions{})
					if err == nil {
						key, _ := cache.MetaNamespaceKeyFunc(workloadObj)
						addToQueue(ownerSD, key, "CronJob")
					}
				}
			}
		}
	}
	deploymentInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return clientset.AppsV1().Deployments("").List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return clientset.AppsV1().Deployments("").Watch(context.TODO(), options)
			},
		},
		&appsv1.Deployment{},
		0,
		cache.Indexers{},
	)
	deploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    workloadAddFunc,
		DeleteFunc: workloadDeleteFunc,
	})
	daemonSetInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return clientset.AppsV1().DaemonSets("").List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return clientset.AppsV1().DaemonSets("").Watch(context.TODO(), options)
			},
		},
		&appsv1.DaemonSet{},
		0,
		cache.Indexers{},
	)
	daemonSetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    workloadAddFunc,
		DeleteFunc: workloadDeleteFunc,
	})
	statefulSetInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return clientset.AppsV1().StatefulSets("").List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return clientset.AppsV1().StatefulSets("").Watch(context.TODO(), options)
			},
		},
		&appsv1.StatefulSet{},
		0,
		cache.Indexers{},
	)
	statefulSetInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    workloadAddFunc,
		DeleteFunc: workloadDeleteFunc,
	})
	jobInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return clientset.BatchV1().Jobs("").List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return clientset.BatchV1().Jobs("").Watch(context.TODO(), options)
			},
		},
		&batchv1.Job{},
		0,
		cache.Indexers{},
	)
	jobInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    workloadAddFunc,
		DeleteFunc: workloadDeleteFunc,
	})
	cronJobInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return clientset.BatchV1beta1().CronJobs("").List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return clientset.BatchV1beta1().CronJobs("").Watch(context.TODO(), options)
			},
		},
		&batchv1beta.CronJob{},
		0,
		cache.Indexers{},
	)
	cronJobInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    workloadAddFunc,
		DeleteFunc: workloadDeleteFunc,
	})
	controller := controller{
		logger:              log.NewEntry(log.New()),
		informer:            informer,
		nodeInformer:        nodeInformer,
		deploymentInformer:  deploymentInformer,
		daemonSetInformer:   daemonSetInformer,
		statefulSetInformer: statefulSetInformer,
		jobInformer:         jobInformer,
		cronJobInformer:     cronJobInformer,
		queue:               queue,
		handler:             sdHandler,
		wg:                  wg,
	}

	// A channel to terminate elegantly
	stopCh := make(chan struct{})
	defer close(stopCh)
	// Run the controller loop as a background task to start processing resources
	go controller.run(stopCh, clientset, edgenetClientset)
	// A channel to observe OS signals for smooth shut down
	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	signal.Notify(sigTerm, syscall.SIGINT)
	<-sigTerm
}

// Run starts the controller loop
func (c *controller) run(stopCh <-chan struct{}, clientset kubernetes.Interface, edgenetClientset versioned.Interface) {
	// A Go panic which includes logging and terminating
	defer utilruntime.HandleCrash()
	// Shutdown after all goroutines have done
	defer c.queue.ShutDown()
	c.logger.Info("run: initiating")
	c.handler.Init(clientset, edgenetClientset)
	// Run the informer to list and watch resources
	go c.informer.Run(stopCh)
	go c.nodeInformer.Run(stopCh)
	go c.deploymentInformer.Run(stopCh)
	go c.daemonSetInformer.Run(stopCh)
	go c.statefulSetInformer.Run(stopCh)

	// Synchronization to settle resources one
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced, c.nodeInformer.HasSynced, c.deploymentInformer.HasSynced, c.daemonSetInformer.HasSynced, c.statefulSetInformer.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Error syncing cache"))
		return
	}
	c.logger.Info("run: cache sync complete")
	// Operate the runWorker
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

// To process new objects added to the queue
func (c *controller) runWorker() {
	log.Info("runWorker: starting")
	// Run processNextItem for all the changes
	for c.processNextItem() {
		log.Info("runWorker: processing next item")
	}

	log.Info("runWorker: completed")
}

// This function deals with the queue and sends each item in it to the specified handler to be processed.
func (c *controller) processNextItem() bool {
	log.Info("processNextItem: start")
	// Fetch the next item of the queue
	event, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(event)
	// Get the key string
	keyRaw := event.(informerevent).key
	// Use the string key to get the object from the indexer
	item, exists, err := c.informer.GetIndexer().GetByKey(keyRaw)
	if err != nil {
		if c.queue.NumRequeues(event.(informerevent).key) < 5 {
			c.logger.Errorf("Controller.processNextItem: Failed processing item with key %s with error %v, retrying", event.(informerevent).key, err)
			c.queue.AddRateLimited(event.(informerevent).key)
		} else {
			c.logger.Errorf("Controller.processNextItem: Failed processing item with key %s with error %v, no more retries", event.(informerevent).key, err)
			c.queue.Forget(event.(informerevent).key)
			utilruntime.HandleError(err)
		}
	}

	if !exists {
		if event.(informerevent).function == delete {
			c.logger.Infof("Controller.processNextItem: object deleted detected: %s", keyRaw)
			c.handler.ObjectDeleted(item)
		}
	} else {
		if event.(informerevent).function == create {
			c.logger.Infof("Controller.processNextItem: object created detected: %s", keyRaw)
			c.handler.ObjectCreated(item)
		} else if event.(informerevent).function == update {
			c.logger.Infof("Controller.processNextItem: object updated detected: %s", keyRaw)
			c.handler.ObjectUpdated(item)
		}
	}
	c.queue.Forget(event.(informerevent).key)

	return true
}
