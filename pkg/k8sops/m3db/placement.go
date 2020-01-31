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

package m3db

import (
	"fmt"
	"strings"
	"text/template"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1alpha1"
	"github.com/m3db/m3db-operator/pkg/k8sops/labels"
	"github.com/m3db/m3db-operator/pkg/k8sops/podidentity"

	"github.com/m3db/m3/src/cluster/generated/proto/placementpb"

	corev1 "k8s.io/api/core/v1"

	pkgerrors "github.com/pkg/errors"
)

const (
	_zoneEmbedded             = "embedded"
	defaultM3DBPort           = 9000
	defaultNodeEndpointFormat = "{{ .PodName }}.{{ .M3DBService }}:{{ .Port }}"
)

type endpointContext struct {
	PodName      string
	M3DBService  string
	PodNamespace string
	Port         uint32
}

// PlacementInstanceFromPod creates a new m3cluster placement instance given a
// pod spec.
func PlacementInstanceFromPod(cluster *myspec.M3DBCluster, pod *corev1.Pod, idProvider podidentity.Provider) (*placementpb.Instance, error) {
	isoGroup, ok := pod.ObjectMeta.Labels[labels.IsolationGroup]
	if !ok {
		return nil, fmt.Errorf("could not find label %s in %v", labels.IsolationGroup, pod.ObjectMeta.Labels)
	}

	id, err := idProvider.Identity(pod, cluster)
	if err != nil {
		return nil, err
	}

	idStr, err := podidentity.IdentityJSON(id)
	if err != nil {
		return nil, err
	}

	epFmt := cluster.Spec.NodeEndpointFormat
	if epFmt == "" {
		epFmt = defaultNodeEndpointFormat
	}

	str := &strings.Builder{}
	epCtx := endpointContext{
		PodName:      pod.Name,
		PodNamespace: pod.Namespace,
		M3DBService:  HeadlessServiceName(cluster.Name),
		Port:         defaultM3DBPort,
	}

	tmpl, err := template.New("nodeEndpoint").Parse(epFmt)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "cannot construct node endpoint template")
	}
	if err := tmpl.Execute(str, epCtx); err != nil {
		return nil, pkgerrors.WithMessage(err, "cannot execute node endpoint template")
	}

	instance := &placementpb.Instance{
		Id:             idStr,
		IsolationGroup: isoGroup,
		Zone:           _zoneEmbedded,
		Weight:         100,
		Hostname:       pod.Name + "." + epCtx.M3DBService,
		Endpoint:       str.String(),
		Port:           defaultM3DBPort,
	}

	return instance, nil
}
