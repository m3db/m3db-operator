package stub

import (
	"context"

	api "github.com/m3db/m3db-operator/pkg/apis/m3db/v1alpha1"
	"github.com/m3db/m3db-operator/pkg/m3db"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
)

func NewHandler() sdk.Handler {
	return &Handler{}
}

type Handler struct{}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	switch o := event.Object.(type) {
	case *api.M3DBService:
		if err := m3db.Reconcile(o); err != nil {
			return err
		}
	}
	return nil
}
