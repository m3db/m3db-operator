package m3db

import (
	api "github.com/m3db/m3db-operator/pkg/apis/m3db/v1alpha1"
)

// Reconcile reconciles the vault cluster's state to the spec specified by vr
// by preparing the TLS secrets, deploying the etcd and vault cluster,
// and finally updating the vault deployment if needed.
func Reconcile(s *api.M3DBService) error {
	s = s.DeepCopy()
	scs, err := getM3DBStatus(s)
	if err != nil {
		return err
	}
	// After first time reconcile, phase will switch to "Running".
	if s.Status.Phase == api.ClusterPhaseInitial {
		return updateM3DBStatus(s, &api.M3DBServiceStatus{Phase: api.ClusterPhaseRunning})
	}
	return updateM3DBStatus(s, scs)
}
