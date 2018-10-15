// Copyright (c) 2018 Uber Technologies, Inc.
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

// Package labels defines constants and helpers for labels used throughout the
// m3db operator.
package labels

import (
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
)

const (
	// App is the label used to identify the application.
	App = "operator.m3db.io/app"
	// AppM3DB is the value for "App" common to all operator-created clusters.
	AppM3DB = "m3db"
	// Cluster is the label identifying what m3db cluster an object is a part of.
	Cluster = "operator.m3db.io/cluster"
	// IsolationGroup identifies what isolation group an object is in.
	IsolationGroup = "operator.m3db.io/isolation-group"
	// StatefulSet identifies what StatefulSet an object belongs to.
	StatefulSet = "operator.m3db.io/stateful-set"
	// Component identifies what component of the M3 stack an object is part of.
	Component = "operator.m3db.io/component"
	// ComponentM3DBNode indicates a component is a dbnode.
	ComponentM3DBNode = "m3dbnode"
	// ComponentCoordinator indicates a component is a coordinator.
	ComponentCoordinator = "coordinator"
)

// BaseLabels returns the base labels we apply to all objects created by the
// operator for a given cluster.
func BaseLabels(cluster *myspec.M3DBCluster) map[string]string {
	base := map[string]string{
		App:     AppM3DB,
		Cluster: cluster.Name,
	}

	for k, v := range cluster.Spec.Labels {
		base[k] = v
	}

	return base
}
