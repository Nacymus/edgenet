/*
Copyright The Kubernetes Authors.

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

package v1alpha

import (
	"context"
	"time"

	v1alpha "github.com/EdgeNet-project/edgenet/pkg/apis/apps/v1alpha"
	scheme "github.com/EdgeNet-project/edgenet/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// AuthoritiesGetter has a method to return a AuthorityInterface.
// A group's client should implement this interface.
type AuthoritiesGetter interface {
	Authorities() AuthorityInterface
}

// AuthorityInterface has methods to work with Authority resources.
type AuthorityInterface interface {
	Create(ctx context.Context, authority *v1alpha.Authority, opts v1.CreateOptions) (*v1alpha.Authority, error)
	Update(ctx context.Context, authority *v1alpha.Authority, opts v1.UpdateOptions) (*v1alpha.Authority, error)
	UpdateStatus(ctx context.Context, authority *v1alpha.Authority, opts v1.UpdateOptions) (*v1alpha.Authority, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha.Authority, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha.AuthorityList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha.Authority, err error)
	AuthorityExpansion
}

// authorities implements AuthorityInterface
type authorities struct {
	client rest.Interface
}

// newAuthorities returns a Authorities
func newAuthorities(c *AppsV1alphaClient) *authorities {
	return &authorities{
		client: c.RESTClient(),
	}
}

// Get takes name of the authority, and returns the corresponding authority object, and an error if there is any.
func (c *authorities) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha.Authority, err error) {
	result = &v1alpha.Authority{}
	err = c.client.Get().
		Resource("authorities").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Authorities that match those selectors.
func (c *authorities) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha.AuthorityList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha.AuthorityList{}
	err = c.client.Get().
		Resource("authorities").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested authorities.
func (c *authorities) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("authorities").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a authority and creates it.  Returns the server's representation of the authority, and an error, if there is any.
func (c *authorities) Create(ctx context.Context, authority *v1alpha.Authority, opts v1.CreateOptions) (result *v1alpha.Authority, err error) {
	result = &v1alpha.Authority{}
	err = c.client.Post().
		Resource("authorities").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(authority).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a authority and updates it. Returns the server's representation of the authority, and an error, if there is any.
func (c *authorities) Update(ctx context.Context, authority *v1alpha.Authority, opts v1.UpdateOptions) (result *v1alpha.Authority, err error) {
	result = &v1alpha.Authority{}
	err = c.client.Put().
		Resource("authorities").
		Name(authority.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(authority).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *authorities) UpdateStatus(ctx context.Context, authority *v1alpha.Authority, opts v1.UpdateOptions) (result *v1alpha.Authority, err error) {
	result = &v1alpha.Authority{}
	err = c.client.Put().
		Resource("authorities").
		Name(authority.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(authority).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the authority and deletes it. Returns an error if one occurs.
func (c *authorities) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("authorities").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *authorities) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("authorities").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched authority.
func (c *authorities) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha.Authority, err error) {
	result = &v1alpha.Authority{}
	err = c.client.Patch(pt).
		Resource("authorities").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
