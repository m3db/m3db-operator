package controller

import (
	"sort"
	"testing"
	"time"

	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"
	"github.com/stretchr/testify/assert"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func newMeta(name string, labels map[string]string) *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:      name,
		Labels:    labels,
		Namespace: "namespace",
	}
}

func TestGetChildStatefulSets(t *testing.T) {
	tests := []struct {
		cluster     *metav1.ObjectMeta
		sets        []*metav1.ObjectMeta
		expChildren []string
	}{
		{
			cluster: newMeta("cluster1", map[string]string{"foo": "bar"}),
			sets: []*metav1.ObjectMeta{
				newMeta("set1", nil),
			},
			expChildren: []string{},
		},
		{
			cluster: newMeta("cluster1", map[string]string{"foo": "bar"}),
			sets: []*metav1.ObjectMeta{
				newMeta("set1", map[string]string{"foo": "bar"}),
			},
			expChildren: []string{"set1"},
		},
	}

	for _, test := range tests {
		cluster := &myspec.M3DBCluster{
			ObjectMeta: *test.cluster,
		}
		cluster.ObjectMeta.UID = "abcd"

		objects := make([]runtime.Object, len(test.sets))
		statefulSets := make([]*appsv1.StatefulSet, len(test.sets))
		for i, s := range test.sets {
			set := &appsv1.StatefulSet{
				ObjectMeta: *s,
			}
			statefulSets[i] = set
			objects[i] = set
		}

		clientSet := kubefake.NewSimpleClientset(objects...)
		informers := kubeinformers.NewSharedInformerFactory(clientSet, 0)
		sets := informers.Apps().V1().StatefulSets()
		lister := sets.Lister()

		stopCh := make(chan struct{})
		defer close(stopCh)
		go informers.Start(stopCh)

		warmCh := make(chan bool)
		go func() {
			warmCh <- cache.WaitForCacheSync(stopCh, sets.Informer().HasSynced)
		}()

		select {
		case success := <-warmCh:
			assert.True(t, success)
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for informer cache")
		}

		c := &Controller{
			statefulSetLister: lister,
		}

		// Ensure that with no owner references we don't act on these sets
		children, err := c.getChildStatefulSets(cluster)
		assert.NoError(t, err)
		assert.Empty(t, children)

		// Now set the owner refs and ensure we pick up the sets.
		for _, set := range statefulSets {
			set.SetOwnerReferences([]metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, schema.GroupVersionKind{
					Group:   myspec.SchemeGroupVersion.Group,
					Version: myspec.SchemeGroupVersion.Version,
					Kind:    "m3dbcluster",
				}),
			})
			_, err := clientSet.AppsV1().StatefulSets("namespace").Update(set)
			assert.NoError(t, err)
		}

		// Informer caches can take a bit to catch up
		var ok bool
		for i := 0; i < 5; i++ {
			children, err = c.getChildStatefulSets(cluster)
			assert.NoError(t, err)
			if len(children) != len(test.expChildren) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			ok = true
		}
		assert.True(t, ok, "expected to fetch sets")

		setNames := make([]string, len(test.expChildren))
		for i, child := range children {
			setNames[i] = child.Name
		}
		sort.Strings(setNames)
		assert.Equal(t, test.expChildren, setNames)
	}
}
