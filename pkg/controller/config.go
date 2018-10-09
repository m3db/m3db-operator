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

package controller

/*
// loadM3Configuration will load the configmap for m3 and unmarshal the data
// into the Configuration struct for m3.
func (c *Controller) loadM3Configuration(cluster *myspec.M3DBCluster) (*m3config.Configuration, error) {
	m3ConfigMap, err := c.k8sclient.kclient.CoreV1().ConfigMaps(cluster.GetNamespace()).Get("m3-configuration", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	m3ConfigMap = m3ConfigMap.DeepCopy()
	// TODO(PS) Move static string to operator spec?
	m3Config := m3ConfigMap.Data["m3.yml"]
	jsonBytes, err := yaml.YAMLToJSON([]byte(m3Config))
	if err != nil {
		return nil, err
	}
	cfg := &m3config.Configuration{}
	// TODO(PS) Move static string to operator spec?
	if err = json.Unmarshal(jsonBytes, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}*/
