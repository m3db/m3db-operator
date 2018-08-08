package k8sops

import (
	"github.com/Sirupsen/logrus"
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	_m3dbNodeSvcName      = "m3dbnode"
	_m3CoordinatorSvcName = "m3coordinator"
)

// DeleteM3DBNodeSvc deletes the m3dbnode service
func (k *K8sops) DeleteM3DBNodeSvc(cluster *myspec.M3DBCluster) error {
	return k.Kclient.CoreV1().Services(cluster.GetNamespace()).Delete(_m3dbNodeSvcName, &metav1.DeleteOptions{})
}

// EnsureM3DBNodeSvc ensures that the m3dbnode service exists
func (k *K8sops) EnsureM3DBNodeSvc(cluster *myspec.M3DBCluster) error {
	svc, err := k.Kclient.CoreV1().Services(cluster.GetNamespace()).Get(_m3dbNodeSvcName, metav1.GetOptions{})
	if len(svc.Name) == 0 {
		m3dbNodeSvc := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: _m3dbNodeSvcName,
				Labels: map[string]string{
					"app": _m3dbNodeSvcName,
				},
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"app": _m3dbNodeSvcName,
				},
				Ports:     k.BuildM3DBNodeSvcPorts(),
				ClusterIP: "None",
			},
		}

		if _, err := k.Kclient.CoreV1().Services(cluster.GetNamespace()).Create(m3dbNodeSvc); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	logrus.Info("ensured m3dbnode service is created")
	return nil
}

func (k *K8sops) BuildM3DBNodeSvcPorts() []v1.ServicePort {
	return []v1.ServicePort{
		v1.ServicePort{
			Name:     "client",
			Port:     9200,
			Protocol: v1.ProtocolTCP,
		},
		v1.ServicePort{
			Name:     "cluster",
			Port:     9201,
			Protocol: v1.ProtocolTCP,
		},
		v1.ServicePort{
			Name:     "http-node",
			Port:     9202,
			Protocol: v1.ProtocolTCP,
		},
		v1.ServicePort{
			Name:     "debug",
			Port:     9204,
			Protocol: v1.ProtocolTCP,
		},
		v1.ServicePort{
			Name:     "query",
			Port:     7201,
			Protocol: v1.ProtocolTCP,
		},
		v1.ServicePort{
			Name:     "query-metrics",
			Port:     7203,
			Protocol: v1.ProtocolTCP,
		},
	}
}

// DeleteM3CoordinatorSvc deletes the m3dbnode service
func (k *K8sops) DeleteM3CoordinatorSvc(cluster *myspec.M3DBCluster) error {
	return k.Kclient.CoreV1().Services(cluster.GetNamespace()).Delete(_m3CoordinatorSvcName, &metav1.DeleteOptions{})
}

// EnsureM3CoordinatorSvc ensures that the m3dbnode service exists
func (k *K8sops) EnsureM3CoordinatorSvc(cluster *myspec.M3DBCluster) error {
	svc, err := k.Kclient.CoreV1().Services(cluster.GetNamespace()).Get(_m3CoordinatorSvcName, metav1.GetOptions{})
	if len(svc.Name) == 0 {
		m3CoordinatorSvc := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: _m3CoordinatorSvcName,
				Labels: map[string]string{
					"app": _m3CoordinatorSvcName,
				},
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"app": _m3dbNodeSvcName,
				},
				Ports: k.BuildM3CoordinatorSvcPorts(),
			},
		}

		if _, err := k.Kclient.CoreV1().Services(cluster.GetNamespace()).Create(m3CoordinatorSvc); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	logrus.Info("ensured m3coordinator service is created")
	return nil
}

// BuildM3CoordinatorSvcPorts creates the m3coordinator service ports
func (k *K8sops) BuildM3CoordinatorSvcPorts() []v1.ServicePort {
	return []v1.ServicePort{
		v1.ServicePort{
			Name:     "query",
			Port:     7201,
			Protocol: v1.ProtocolTCP,
		},
		v1.ServicePort{
			Name:     "query-metrics",
			Port:     7203,
			Protocol: v1.ProtocolTCP,
		},
	}
}
