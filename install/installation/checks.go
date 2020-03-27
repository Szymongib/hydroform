package installation

import (
	"fmt"

	installationTyped "github.com/kyma-project/kyma/components/kyma-operator/pkg/client/clientset/versioned/typed/installer/v1alpha1"

	installationClientset "github.com/kyma-project/kyma/components/kyma-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type InstallationChecker struct {
	k8sClient          kubernetes.Interface
	installationClient installationTyped.InstallationInterface
}

func NewInstallationChecker(kubeconfig *rest.Config) (*InstallationChecker, error) {
	k8sClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %s", err.Error())
	}

	installationClient, err := installationClientset.NewForConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation client: %s", err.Error())
	}

	return &InstallationChecker{
		k8sClient:          k8sClient,
		installationClient: installationClient.InstallerV1alpha1().Installations(defaultInstallationResourceNamespace),
	}, nil
}

func (c *InstallationChecker) IsTillerDeployed() (bool, error) {
	podList, err := c.listTillerPods()
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("error listing tiller pods: %s", err.Error())
	}

	return len(podList.Items) > 0, nil
}

func (c *InstallationChecker) IsTillerReady() (bool, error) {
	podList, err := c.listTillerPods()
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, fmt.Errorf("error no tiller pods found")
		}
		return false, fmt.Errorf("error listing tiller pods: %s", err.Error())
	}

	// pod does not exists, retry
	if len(podList.Items) == 0 {
		return false, nil
	}

	for _, pod := range podList.Items {
		// if any pod is not in the desired status no need to check further
		if corev1.PodRunning != pod.Status.Phase {
			return false, nil
		}
	}

	return true, nil
}

func (c *InstallationChecker) listTillerPods() (*corev1.PodList, error) {
	podClient := c.k8sClient.CoreV1().Pods(kubeSystemNamespace)

	pods, err := podClient.List(v1.ListOptions{LabelSelector: tillerLabelSelector})
	if err != nil {
		return nil, err
	}

	return pods, nil
}

func (c *InstallationChecker) CheckInstallationState() (InstallationState, error) {
	installationCR, err := c.installationClient.Get(kymaInstallationName, metav1.GetOptions{})
	if err != nil {
		return InstallationState{}, err
	}

	return getInstallationState(*installationCR)
}
