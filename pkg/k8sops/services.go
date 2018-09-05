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

package k8sops

import (
	myspec "github.com/m3db/m3db-operator/pkg/apis/m3dboperator/v1"

	"go.uber.org/zap"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetService simply gets a service by name
func (k *K8sops) GetService(cluster *myspec.M3DBCluster, name string) (*v1.Service, error) {
	service, err := k.Kclient.CoreV1().Services(cluster.GetNamespace()).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return service, nil
}

// DeleteService simply deletes a service by name
func (k *K8sops) DeleteService(cluster *myspec.M3DBCluster, name string) error {
	k.logger.Info("deleting service", zap.String("service", name))
	return k.Kclient.CoreV1().Services(cluster.GetNamespace()).Delete(name, &metav1.DeleteOptions{})
}

// EnsureService will create a service by name if it doesn't exist
func (k *K8sops) EnsureService(cluster *myspec.M3DBCluster, svcCfg myspec.ServiceConfiguration) error {
	_, err := k.GetService(cluster, svcCfg.Name)
	if errors.IsNotFound(err) {
		k.logger.Info("service doesn't exist, creating it", zap.String("service", svcCfg.Name))
		svc := k.GenerateService(svcCfg)
		k.logger.Info("service", zap.Any("obj", svc))
		if _, err := k.Kclient.CoreV1().Services(cluster.GetNamespace()).Create(svc); err != nil {
			return err
		}
		k.logger.Info("ensured service is created", zap.String("service", svc.GetName()))
	} else if errors.IsAlreadyExists(err) {
		return nil
	} else if err != nil {
		return err
	}
	return nil
}
