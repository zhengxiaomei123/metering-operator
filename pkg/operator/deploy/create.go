package deploy

import (
	"fmt"
	"path/filepath"

	meteringv1 "github.com/operator-framework/operator-metering/pkg/apis/metering/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/api/core/v1"
)

func (deploy *Deployer) createNamespace() error {
	_, err := deploy.Client.CoreV1().Namespaces().Get(deploy.Namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// TODO: mirror the annotation logic in hack/openshift-install.sh
		namespaceObj := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: deploy.Namespace},
		}

		_, err := deploy.Client.CoreV1().Namespaces().Create(namespaceObj)
		if err != nil {
			return fmt.Errorf("Failed to create %s namespace: %v", deploy.Namespace, err)
		}
		deploy.Logger.Infof("Created the %s namespace", deploy.Namespace)
	} else if err == nil {
		deploy.Logger.Infof("The %s namespace already exists", deploy.Namespace)
	} else {
		return err
	}

	return nil
}

func (deploy *Deployer) createMeteringConfig() error {
	var res meteringv1.MeteringConfig

	err := decodeYAMLManifestToObject(deploy.MeteringCR, &res)
	if err != nil {
		return fmt.Errorf("Failed to decode the YAML manifest: %v", err)
	}

	mc, err := deploy.MeteringClient.MeteringConfigs(deploy.Namespace).Get("operator-metering", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = deploy.MeteringClient.MeteringConfigs(deploy.Namespace).Create(&res)
		if err != nil {
			return fmt.Errorf("Failed to create the MeteringConfig resource: %v", err)
		}
		deploy.Logger.Infof("Created the MeteringConfig resource")
	} else if err == nil {
		mc.Spec = res.Spec

		_, err = deploy.MeteringClient.MeteringConfigs(deploy.Namespace).Update(mc)
		if err != nil {
			return fmt.Errorf("Failed to update the MeteringConfig: %v", err)
		}
		deploy.Logger.Infof("The MeteringConfig resource has been updated")
	} else {
		return err
	}

	return nil
}

func (deploy *Deployer) createMeteringResources() error {
	err := deploy.createMeteringDeployment(filepath.Join(deploy.ManifestLocation, meteringDeploymentFile))
	if err != nil {
		return fmt.Errorf("Failed to create the metering deployment: %v", err)
	}

	err = deploy.createMeteringServiceAccount(filepath.Join(deploy.ManifestLocation, meteringServiceAccountFile))
	if err != nil {
		return fmt.Errorf("Failed to create the metering service account: %v", err)
	}

	err = deploy.createMeteringRole(filepath.Join(deploy.ManifestLocation, meteringRoleFile))
	if err != nil {
		return fmt.Errorf("Failed to create the metering role: %v", err)
	}

	err = deploy.createMeteringRoleBinding(filepath.Join(deploy.ManifestLocation, meteringRoleBindingFile))
	if err != nil {
		return fmt.Errorf("Failed to create the metering role binding: %v", err)
	}

	err = deploy.createMeteringClusterRole(filepath.Join(deploy.ManifestLocation, meteringClusterRoleFile))
	if err != nil {
		return fmt.Errorf("Failed to create the metering cluster role: %v", err)
	}

	err = deploy.createMeteringClusterRoleBinding(filepath.Join(deploy.ManifestLocation, meteringClusterRoleBindingFile))
	if err != nil {
		return fmt.Errorf("Failed to create the metering cluster role binding: %v", err)
	}

	return nil
}

func (deploy *Deployer) createMeteringDeployment(deploymentName string) error {
	var res appsv1.Deployment

	err := decodeYAMLManifestToObject(deploymentName, &res)
	if err != nil {
		return fmt.Errorf("Failed to decode the metering YAML manifest: %v", err)
	}

	// check if the metering operator image needs to be updated
	// TODO: implement support for METERING_OPERATOR_ALL_NAMESPACES and METERING_OPERATOR_TARGET_NAMESPACES
	if deploy.Repo != "" && deploy.Tag != "" {
		newImage := deploy.Repo + ":" + deploy.Tag

		for index := range res.Spec.Template.Spec.Containers {
			res.Spec.Template.Spec.Containers[index].Image = newImage
		}

		deploy.Logger.Infof("Overriding the default image with %s", newImage)
	}

	deployment, err := deploy.Client.AppsV1().Deployments(deploy.Namespace).Get(res.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := deploy.Client.AppsV1().Deployments(deploy.Namespace).Create(&res)
		if err != nil {
			return fmt.Errorf("Failed to create the metering deployment: %v", err)
		}
		deploy.Logger.Infof("Created the metering deployment")
	} else if err == nil {
		deployment.Spec = res.Spec

		_, err = deploy.Client.AppsV1().Deployments(deploy.Namespace).Update(deployment)
		if err != nil {
			return fmt.Errorf("Failed to update the metering deployment: %v", err)
		}
		deploy.Logger.Infof("The metering deployment resource has been updated")
	} else {
		return err
	}

	return nil
}

func (deploy *Deployer) createMeteringServiceAccount(serviceAccountPath string) error {
	var res corev1.ServiceAccount

	err := decodeYAMLManifestToObject(serviceAccountPath, &res)
	if err != nil {
		return fmt.Errorf("Failed to decode the YAML manifest: %v", err)
	}

	_, err = deploy.Client.CoreV1().ServiceAccounts(deploy.Namespace).Get(res.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := deploy.Client.CoreV1().ServiceAccounts(deploy.Namespace).Create(&res)
		if err != nil {
			return fmt.Errorf("Failed to create the metering serviceaccount: %v", err)
		}
		deploy.Logger.Infof("Created the metering serviceaccount")
	} else if err == nil {
		deploy.Logger.Infof("The metering service account already exists")
	} else {
		return err
	}

	return nil
}

func (deploy *Deployer) createMeteringRoleBinding(roleBindingPath string) error {
	var res rbacv1.RoleBinding

	err := decodeYAMLManifestToObject(roleBindingPath, &res)
	if err != nil {
		return fmt.Errorf("Failed to decode the YAML manifest: %v", err)
	}

	// TODO: implement support for METERING_OPERATOR_TARGET_NAMESPACES
	res.Name = deploy.Namespace + "-" + res.Name
	res.RoleRef.Name = res.Name
	res.Namespace = deploy.Namespace

	for index := range res.Subjects {
		res.Subjects[index].Namespace = deploy.Namespace
	}

	_, err = deploy.Client.RbacV1().RoleBindings(deploy.Namespace).Get(res.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := deploy.Client.RbacV1().RoleBindings(deploy.Namespace).Create(&res)
		if err != nil {
			return fmt.Errorf("Failed to create the metering role binding: %v", err)
		}
		deploy.Logger.Infof("Created the metering role binding")
	} else if err == nil {
		deploy.Logger.Infof("The metering role binding already exists")
	} else {
		return err
	}

	return nil
}

func (deploy *Deployer) createMeteringRole(rolePath string) error {
	var res rbacv1.Role

	err := decodeYAMLManifestToObject(rolePath, &res)
	if err != nil {
		return fmt.Errorf("Failed to decode the YAML manifest: %v", err)
	}

	res.Name = deploy.Namespace + "-" + res.Name
	res.Namespace = deploy.Namespace

	_, err = deploy.Client.RbacV1().Roles(deploy.Namespace).Get(res.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := deploy.Client.RbacV1().Roles(deploy.Namespace).Create(&res)
		if err != nil {
			return fmt.Errorf("Failed to create the metering role: %v", err)
		}
		deploy.Logger.Infof("Created the metering role")
	} else if err == nil {
		deploy.Logger.Infof("The metering role already exists")
	} else {
		return err
	}

	return nil
}

func (deploy *Deployer) createMeteringClusterRoleBinding(clusterrolebindingFile string) error {
	var res rbacv1.ClusterRoleBinding

	err := decodeYAMLManifestToObject(clusterrolebindingFile, &res)
	if err != nil {
		return fmt.Errorf("Failed to decode the YAML manifest: %v", err)
	}

	res.Name = deploy.Namespace + "-" + res.Name
	res.RoleRef.Name = res.Name

	for index := range res.Subjects {
		res.Subjects[index].Namespace = deploy.Namespace
	}

	_, err = deploy.Client.RbacV1().ClusterRoleBindings().Get(res.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := deploy.Client.RbacV1().ClusterRoleBindings().Create(&res)
		if err != nil {
			return fmt.Errorf("Failed to create the metering cluster role, got: %v", err)
		}
		deploy.Logger.Infof("Created the metering cluster role binding")
	} else if err == nil {
		deploy.Logger.Infof("The metering cluster role binding already exists")
	} else {
		return err
	}

	return nil
}

func (deploy *Deployer) createMeteringClusterRole(clusterrolePath string) error {
	var res rbacv1.ClusterRole

	err := decodeYAMLManifestToObject(clusterrolePath, &res)
	if err != nil {
		return fmt.Errorf("Failed to decode the YAML manifest: %v", err)
	}

	res.Name = deploy.Namespace + "-" + res.Name

	_, err = deploy.Client.RbacV1().ClusterRoles().Get(res.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := deploy.Client.RbacV1().ClusterRoles().Create(&res)
		if err != nil {
			return fmt.Errorf("Failed to create the metering cluster role: %v", err)
		}
		deploy.Logger.Infof("Created the metering cluster role")
	} else if err == nil {
		deploy.Logger.Infof("The metering cluster role already exists")
	} else {
		return err
	}

	return nil
}

func (deploy *Deployer) createMeteringCRDs() error {
	for _, crd := range deploy.CRDs {
		err := deploy.createMeteringCRD(crd)
		if err != nil {
			return fmt.Errorf("Failed to create a CRD while looping: %v", err)
		}
	}

	return nil
}

func (deploy *Deployer) createMeteringCRD(resource CRD) error {
	err := decodeYAMLManifestToObject(resource.Path, resource.CRD)
	if err != nil {
		return fmt.Errorf("Failed to decode the YAML manifest: %v", err)
	}

	crd, err := deploy.APIExtClient.CustomResourceDefinitions().Get(resource.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := deploy.APIExtClient.CustomResourceDefinitions().Create(resource.CRD)
		if err != nil {
			return fmt.Errorf("Failed to create the %s CRD: %v", resource.CRD.Name, err)
		}
		deploy.Logger.Infof("Created the %s CRD", resource.Name)
	} else if err == nil {
		crd.Spec = resource.CRD.Spec

		_, err := deploy.APIExtClient.CustomResourceDefinitions().Update(crd)
		if err != nil {
			return fmt.Errorf("Failed to update the %s CRD: %v", resource.CRD.Name, err)
		}
		deploy.Logger.Infof("Updated the %s CRD", resource.CRD.Name)
	} else {
		return err
	}

	return nil
}
