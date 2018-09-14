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
