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

package podidentity

import (
	"encoding/json"
	"errors"
	"fmt"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	corev1 "k8s.io/api/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"

	"go.uber.org/zap"
)

const (
	// AnnotationKeyPodIdentity is the annotation key used for pod identity.
	AnnotationKeyPodIdentity = "operator.m3db.io/pod-identity"
)

var (
	errEmptyPodSourceName  = errors.New("pod name cannot by empty with id source == name")
	errEmptyPodSourceUID   = errors.New("pod UID cannot be empty with id source == UID")
	errEmptyNodeExternalID = errors.New("node external ID cannot be empty with source == externalID")
	errEmptyNodeProviderID = errors.New("node provider ID cannot be empty with source == prodiverID")
)

// Provider creates a pod's cluster identity given required info.
type Provider interface {
	Identity(pod *corev1.Pod, cluster *myspec.M3DBCluster) (*myspec.PodIdentity, error)
}

// NewProvider returns a new provider.
func NewProvider(opts ...Option) (Provider, error) {
	pOpts := &options{
		logger: zap.NewNop(),
	}

	for _, o := range opts {
		o.execute(pOpts)
	}

	if err := pOpts.validate(); err != nil {
		return nil, err
	}

	return &provider{
		logger:     pOpts.logger,
		nodeLister: pOpts.nodeLister,
	}, nil
}

type provider struct {
	logger     *zap.Logger
	nodeLister corelisters.NodeLister
}

// Identity returns a pod's identity.
func (p *provider) Identity(pod *corev1.Pod, cluster *myspec.M3DBCluster) (*myspec.PodIdentity, error) {
	config := cluster.Spec.PodIdentityConfig
	if config == nil {
		config = &myspec.PodIdentityConfig{
			Sources: []myspec.PodIdentitySource{
				myspec.PodIdentitySourcePodUID,
			},
		}
	}

	if pod.Name == "" {
		return nil, errEmptyPodSourceName
	}

	// Identity always includes pod name, as we depend on this when replacing
	// instances.
	id := &myspec.PodIdentity{
		Name: pod.Name,
	}

	for _, source := range config.Sources {
		switch source {
		case myspec.PodIdentitySourcePodUID:
			if pod.UID == "" {
				return nil, errEmptyPodSourceUID
			}
			id.UID = string(pod.UID)
		case myspec.PodIdentitySourceNodeSpecExternalID:
			node, err := p.nodeForPod(pod)
			if err != nil {
				return nil, err
			}
			if node.Spec.DoNotUse_ExternalID == "" {
				return nil, errEmptyNodeExternalID
			}
			id.NodeExternalID = node.Spec.DoNotUse_ExternalID
		case myspec.PodIdentitySourceNodeSpecProviderID:
			node, err := p.nodeForPod(pod)
			if err != nil {
				return nil, err
			}
			if node.Spec.ProviderID == "" {
				return nil, errEmptyNodeProviderID
			}
			id.NodeProviderID = node.Spec.ProviderID
		case myspec.PodIdentitySourceNodeName:
			node, err := p.nodeForPod(pod)
			if err != nil {
				return nil, err
			}
			// We don't check for empty node name because we assume objects were
			// validated by the Kubernetes API.
			id.NodeName = node.Name
		default:
			return nil, fmt.Errorf("unrecognized pod identity source %s", source)
		}
	}

	return id, nil
}

func (p *provider) nodeForPod(pod *corev1.Pod) (*corev1.Node, error) {
	podNode := pod.Spec.NodeName
	if podNode == "" {
		p.logger.Warn("pod not yet scheduled", zap.String("pod", pod.Name))
		return nil, fmt.Errorf("pod %s not yet scheduled", pod.Name)
	}

	node, err := p.nodeLister.Get(podNode)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// IdentityJSON returns a pod's identity in JSON form as a string.
func IdentityJSON(id *myspec.PodIdentity) (string, error) {
	data, err := json.Marshal(id)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
