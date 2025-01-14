/*
Copyright 2021 The Everoute Authors.

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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	securityv1alpha1 "github.com/everoute/everoute/pkg/apis/security/v1alpha1"
	clientset "github.com/everoute/everoute/pkg/client/clientset_generated/clientset"
	internalinterfaces "github.com/everoute/everoute/pkg/client/informers_generated/externalversions/internalinterfaces"
	v1alpha1 "github.com/everoute/everoute/pkg/client/listers_generated/security/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// GlobalPolicyInformer provides access to a shared informer and lister for
// GlobalPolicies.
type GlobalPolicyInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.GlobalPolicyLister
}

type globalPolicyInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewGlobalPolicyInformer constructs a new informer for GlobalPolicy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewGlobalPolicyInformer(client clientset.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredGlobalPolicyInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredGlobalPolicyInformer constructs a new informer for GlobalPolicy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredGlobalPolicyInformer(client clientset.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SecurityV1alpha1().GlobalPolicies().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SecurityV1alpha1().GlobalPolicies().Watch(context.TODO(), options)
			},
		},
		&securityv1alpha1.GlobalPolicy{},
		resyncPeriod,
		indexers,
	)
}

func (f *globalPolicyInformer) defaultInformer(client clientset.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredGlobalPolicyInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *globalPolicyInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&securityv1alpha1.GlobalPolicy{}, f.defaultInformer)
}

func (f *globalPolicyInformer) Lister() v1alpha1.GlobalPolicyLister {
	return v1alpha1.NewGlobalPolicyLister(f.Informer().GetIndexer())
}
