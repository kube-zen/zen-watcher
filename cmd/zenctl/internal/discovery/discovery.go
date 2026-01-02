package discovery

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"
)

// ResourceResolver resolves GVK to GVR using discovery
type ResourceResolver struct {
	mapper meta.RESTMapper
}

// NewResourceResolver creates a new resource resolver
func NewResourceResolver(discClient discovery.DiscoveryInterface) (*ResourceResolver, error) {
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discClient))
	return &ResourceResolver{mapper: mapper}, nil
}

// ResolveGVR resolves a GroupVersionKind to GroupVersionResource
func (r *ResourceResolver) ResolveGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	// RESTMapper.KindFor expects GVR, but we have GVK
	// Use RESTMapping which takes GVK and returns both GVK and GVR
	mapping, err := r.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to resolve GVR for %s: %w", gvk, err)
	}
	return mapping.Resource, nil
}

// ExpectedGVKs defines the expected GroupVersionKinds we're looking for
var ExpectedGVKs = map[string]schema.GroupVersionKind{
	"DeliveryFlow": {
		Group:   "routing.zen.kube-zen.io",
		Version: "v1alpha1",
		Kind:    "DeliveryFlow",
	},
	"Destination": {
		Group:   "routing.zen.kube-zen.io",
		Version: "v1alpha1",
		Kind:    "Destination",
	},
	"Ingester": {
		Group:   "zen.kube-zen.io",
		Version: "v1alpha1",
		Kind:    "Ingester",
	},
}

// ResolveAll resolves all expected GVKs to GVRs
func (r *ResourceResolver) ResolveAll() (map[string]schema.GroupVersionResource, map[string]error) {
	gvrs := make(map[string]schema.GroupVersionResource)
	errors := make(map[string]error)

	for name, gvk := range ExpectedGVKs {
		gvr, err := r.ResolveGVR(gvk)
		if err != nil {
			errors[name] = err
		} else {
			gvrs[name] = gvr
		}
	}

	return gvrs, errors
}

