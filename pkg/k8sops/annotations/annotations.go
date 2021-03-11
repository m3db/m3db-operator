// Copyright (c) 2019 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package annotations defines constants and helpers for annotations used
// throughout the m3db operator.
package annotations

import (
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"
)

const (
	// App is the label used to identify the application.
	App = labels.App
	// AppM3DB is the value for "App" common to all operator-created clusters.
	AppM3DB = labels.AppM3DB
	// Cluster is the label identifying what m3db cluster an object is a part of.
	Cluster = labels.Cluster
	// Update is the annotation used by the operator to determine if a StatefulSet is
	// allowed to be updated.
	Update = "operator.m3db.io/update"
	// ParallelUpdate is the annotation used by the operator to determine if a StatefulSet
	// is allowed to be updated in parallel.
	ParallelUpdate = "operator.m3db.io/parallel-update"
	// ParallelUpdateInProgress is the annotation used by the operator indicate a parallel update
	// is underway. This annotation should only be used by the operator.
	ParallelUpdateInProgress = "operator.m3db.io/parallel-update-in-progress"
	// EnabledVal is the value that indicates an annotation is enabled.
	EnabledVal = "enabled"
)

// BaseAnnotations returns the base annotations we apply to all objects
// created by the operator for a given cluster.
func BaseAnnotations(cluster *myspec.M3DBCluster) map[string]string {
	base := map[string]string{
		App:     AppM3DB,
		Cluster: cluster.Name,
	}
	if configured := cluster.Spec.Annotations; configured != nil {
		for k, v := range configured {
			base[k] = v
		}
	}

	return base
}

// PodAnnotations is for specifying annotations that are only to be
// applied to the pods such as prometheus scrape tags
func PodAnnotations(cluster *myspec.M3DBCluster) map[string]string {
	base := BaseAnnotations(cluster)
	for k := range cluster.Spec.PodMetadata.Annotations {
		// accept any user-specified annotations if its safe to do so
		if _, found := base[k]; !found {
			base[k] = cluster.Spec.PodMetadata.Annotations[k]
		}
	}

	return base
}
