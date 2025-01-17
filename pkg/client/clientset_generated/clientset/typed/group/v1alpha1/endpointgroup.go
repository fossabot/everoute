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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/everoute/everoute/pkg/apis/group/v1alpha1"
	scheme "github.com/everoute/everoute/pkg/client/clientset_generated/clientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// EndpointGroupsGetter has a method to return a EndpointGroupInterface.
// A group's client should implement this interface.
type EndpointGroupsGetter interface {
	EndpointGroups() EndpointGroupInterface
}

// EndpointGroupInterface has methods to work with EndpointGroup resources.
type EndpointGroupInterface interface {
	Create(ctx context.Context, endpointGroup *v1alpha1.EndpointGroup, opts v1.CreateOptions) (*v1alpha1.EndpointGroup, error)
	Update(ctx context.Context, endpointGroup *v1alpha1.EndpointGroup, opts v1.UpdateOptions) (*v1alpha1.EndpointGroup, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.EndpointGroup, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.EndpointGroupList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.EndpointGroup, err error)
	EndpointGroupExpansion
}

// endpointGroups implements EndpointGroupInterface
type endpointGroups struct {
	client rest.Interface
}

// newEndpointGroups returns a EndpointGroups
func newEndpointGroups(c *GroupV1alpha1Client) *endpointGroups {
	return &endpointGroups{
		client: c.RESTClient(),
	}
}

// Get takes name of the endpointGroup, and returns the corresponding endpointGroup object, and an error if there is any.
func (c *endpointGroups) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.EndpointGroup, err error) {
	result = &v1alpha1.EndpointGroup{}
	err = c.client.Get().
		Resource("endpointgroups").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of EndpointGroups that match those selectors.
func (c *endpointGroups) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.EndpointGroupList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.EndpointGroupList{}
	err = c.client.Get().
		Resource("endpointgroups").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested endpointGroups.
func (c *endpointGroups) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("endpointgroups").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a endpointGroup and creates it.  Returns the server's representation of the endpointGroup, and an error, if there is any.
func (c *endpointGroups) Create(ctx context.Context, endpointGroup *v1alpha1.EndpointGroup, opts v1.CreateOptions) (result *v1alpha1.EndpointGroup, err error) {
	result = &v1alpha1.EndpointGroup{}
	err = c.client.Post().
		Resource("endpointgroups").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(endpointGroup).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a endpointGroup and updates it. Returns the server's representation of the endpointGroup, and an error, if there is any.
func (c *endpointGroups) Update(ctx context.Context, endpointGroup *v1alpha1.EndpointGroup, opts v1.UpdateOptions) (result *v1alpha1.EndpointGroup, err error) {
	result = &v1alpha1.EndpointGroup{}
	err = c.client.Put().
		Resource("endpointgroups").
		Name(endpointGroup.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(endpointGroup).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the endpointGroup and deletes it. Returns an error if one occurs.
func (c *endpointGroups) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("endpointgroups").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *endpointGroups) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("endpointgroups").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched endpointGroup.
func (c *endpointGroups) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.EndpointGroup, err error) {
	result = &v1alpha1.EndpointGroup{}
	err = c.client.Patch(pt).
		Resource("endpointgroups").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
