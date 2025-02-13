// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package maturity

import (
	"strings"

	corev1 "k8s.io/api/core/v1"

	"istio.io/api/annotation"
	"istio.io/istio/pkg/config/analysis"
	"istio.io/istio/pkg/config/analysis/analyzers/util"
	"istio.io/istio/pkg/config/analysis/msg"
	"istio.io/istio/pkg/config/resource"
	"istio.io/istio/pkg/config/schema/collection"
	"istio.io/istio/pkg/config/schema/collections"
	"istio.io/istio/pkg/config/schema/gvk"
)

// AlphaAnalyzer checks for alpha Istio annotations in K8s resources
type AlphaAnalyzer struct{}

// the alpha analyzer is currently explicitly left out of the default collection of analyzers to run, as it results
// in too much noise for users, with annotations that are set by default.  Once the noise dies down, this should be
// added to the CombinedAnalyzers() function.

var istioAnnotations = annotation.AllResourceAnnotations()

// Metadata implements analyzer.Analyzer
func (*AlphaAnalyzer) Metadata() analysis.Metadata {
	return analysis.Metadata{
		Name:        "annotations.AlphaAnalyzer",
		Description: "Checks for alpha Istio annotations in Kubernetes resources",
		Inputs: collection.Names{
			collections.K8SCoreV1Namespaces.Name(),
			collections.K8SCoreV1Services.Name(),
			collections.K8SCoreV1Pods.Name(),
			collections.K8SAppsV1Deployments.Name(),
		},
	}
}

var ignoredAnnotations = map[string]bool{
	annotation.SidecarInterceptionMode.Name:               true,
	annotation.SidecarTrafficIncludeInboundPorts.Name:     true,
	annotation.SidecarTrafficExcludeInboundPorts.Name:     true,
	annotation.SidecarTrafficIncludeOutboundIPRanges.Name: true,
}

// Analyze implements analysis.Analyzer
func (fa *AlphaAnalyzer) Analyze(ctx analysis.Context) {
	ctx.ForEach(collections.K8SCoreV1Namespaces.Name(), func(r *resource.Instance) bool {
		fa.allowAnnotations(r, ctx, collections.K8SCoreV1Namespaces.Name())
		return true
	})
	ctx.ForEach(collections.K8SCoreV1Services.Name(), func(r *resource.Instance) bool {
		fa.allowAnnotations(r, ctx, collections.K8SCoreV1Services.Name())
		return true
	})
	ctx.ForEach(collections.K8SCoreV1Pods.Name(), func(r *resource.Instance) bool {
		fa.allowAnnotations(r, ctx, collections.K8SCoreV1Pods.Name())
		return true
	})
	ctx.ForEach(collections.K8SAppsV1Deployments.Name(), func(r *resource.Instance) bool {
		fa.allowAnnotations(r, ctx, collections.K8SAppsV1Deployments.Name())
		return true
	})
}

func (*AlphaAnalyzer) allowAnnotations(r *resource.Instance, ctx analysis.Context, collectionType collection.Name) {
	if len(r.Metadata.Annotations) == 0 {
		return
	}

	var shouldSkipDefault bool
	if r.Metadata.Schema.GroupVersionKind() == gvk.Pod {
		shouldSkipDefault = isCNIEnabled(r.Message.(*corev1.PodSpec))
	}

	// It is fine if the annotation is kubectl.kubernetes.io/last-applied-configuration.
	for ann := range r.Metadata.Annotations {
		if !istioAnnotation(ann) {
			continue
		}

		if annotationDef := lookupAnnotation(ann); annotationDef != nil {
			if annotationDef.FeatureStatus == annotation.Alpha {
				// this annotation is set by default in istiod, don't alert on it.
				if annotationDef.Name == annotation.SidecarStatus.Name {
					continue
				}
				// some annotations are set by default in istiod, don't alert on it.
				if shouldSkipDefault && ignoredAnnotations[annotationDef.Name] {
					continue
				}
				m := msg.NewAlphaAnnotation(r, ann)
				util.AddLineNumber(r, ann, m)

				ctx.Report(collectionType, m)
			}
		}
	}
}

func isCNIEnabled(pod *corev1.PodSpec) bool {
	var hasIstioInitContainer bool
	for _, c := range pod.InitContainers {
		if c.Name == "istio-init" {
			hasIstioInitContainer = true
			break
		}
	}
	return !hasIstioInitContainer
}

// istioAnnotation is true if the annotation is in Istio's namespace
func istioAnnotation(ann string) bool {
	// We document this Kubernetes annotation, we should analyze it as well
	if ann == "kubernetes.io/ingress.class" {
		return true
	}

	parts := strings.Split(ann, "/")
	if len(parts) == 0 {
		return false
	}

	if !strings.HasSuffix(parts[0], "istio.io") {
		return false
	}

	return true
}

func lookupAnnotation(ann string) *annotation.Instance {
	for _, candidate := range istioAnnotations {
		if candidate.Name == ann {
			return candidate
		}
	}

	return nil
}
