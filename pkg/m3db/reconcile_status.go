package m3db

import (
	"reflect"

	api "github.com/m3db/m3db-operator/pkg/apis/m3db/v1alpha1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
)

func updateM3DBStatus(s *api.M3DBService, status *api.M3DBServiceStatus) error {
	// don't update the status if there aren't any changes.
	if reflect.DeepEqual(s.Status, *status) {
		return nil
	}
	s.Status = *status
	return sdk.Update(s)
}

func getM3DBStatus(s *api.M3DBService) (*api.M3DBServiceStatus, error) {
	return &api.M3DBServiceStatus{
		Phase: api.ClusterPhaseRunning,
	}, nil
}
