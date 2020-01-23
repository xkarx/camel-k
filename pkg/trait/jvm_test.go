/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package trait

import (
	"context"
	"sort"
	"testing"

	"github.com/scylladb/go-set/strset"
	"github.com/stretchr/testify/assert"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	serving "knative.dev/serving/pkg/apis/serving/v1"

	v1 "github.com/apache/camel-k/pkg/apis/camel/v1"
	"github.com/apache/camel-k/pkg/util/kubernetes"
	"github.com/apache/camel-k/pkg/util/test"
)

func TestConfigureJvmTraitInRightPhasesDoesSucceed(t *testing.T) {
	trait, environment := createNominalJvmTest()

	configured, err := trait.Configure(environment)
	assert.Nil(t, err)
	assert.True(t, configured)
}

func TestConfigureJvmTraitInWrongIntegrationPhaseDoesNotSucceed(t *testing.T) {
	trait, environment := createNominalJvmTest()
	environment.Integration.Status.Phase = v1.IntegrationPhaseError

	configured, err := trait.Configure(environment)
	assert.Nil(t, err)
	assert.False(t, configured)
}

func TestConfigureJvmTraitInWrongIntegrationKitPhaseDoesNotSucceed(t *testing.T) {
	trait, environment := createNominalJvmTest()
	environment.IntegrationKit.Status.Phase = v1.IntegrationKitPhaseWaitingForPlatform

	configured, err := trait.Configure(environment)
	assert.Nil(t, err)
	assert.False(t, configured)
}

func TestConfigureJvmDisabledTraitDoesNotSucceed(t *testing.T) {
	trait, environment := createNominalJvmTest()
	trait.Enabled = new(bool)

	configured, err := trait.Configure(environment)
	assert.Nil(t, err)
	assert.False(t, configured)
}

func TestApplyJvmTraitWithDeploymentResource(t *testing.T) {
	trait, environment := createNominalJvmTest()

	d := appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: defaultContainerName,
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/mount/path",
								},
							},
						},
					},
				},
			},
		},
	}

	environment.Resources.Add(&d)

	err := trait.Apply(environment)

	assert.Nil(t, err)

	cp := strset.New("/etc/camel/resources", "./resources", "/mount/path").List()
	sort.Strings(cp)

	assert.Equal(t, d.Spec.Template.Spec.Containers[0].Args, []string{
		"-cp",
		"./resources:/etc/camel/resources:/mount/path",
		"org.apache.camel.k.main.Application",
	})
}

func TestApplyJvmTraitWithKNativeResource(t *testing.T) {
	trait, environment := createNominalJvmTest()

	s := serving.Service{}
	s.Spec.ConfigurationSpec.Template = serving.RevisionTemplateSpec{}
	s.Spec.ConfigurationSpec.Template.Spec.Containers = []corev1.Container{
		{
			Name: defaultContainerName,
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: "/mount/path",
				},
			},
		},
	}

	environment.Resources.Add(&s)

	err := trait.Apply(environment)

	assert.Nil(t, err)

	cp := strset.New("/etc/camel/resources", "./resources", "/mount/path").List()
	sort.Strings(cp)

	assert.Equal(t, s.Spec.Template.Spec.Containers[0].Args, []string{
		"-cp",
		"./resources:/etc/camel/resources:/mount/path",
		"org.apache.camel.k.main.Application",
	})
}

func createNominalJvmTest() (*jvmTrait, *Environment) {
	return createJvmTestWithKitType(v1.IntegrationKitTypePlatform)
}

func createJvmTestWithKitType(kitType string) (*jvmTrait, *Environment) {
	client, _ := test.NewFakeClient(
		&v1.IntegrationKit{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       v1.IntegrationKindKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "kit-namespace",
				Name:      "kit-name",
				Labels: map[string]string{
					"camel.apache.org/kit.type": kitType,
				},
			},
		},
	)

	trait := newJvmTrait()
	enabled := true
	trait.Enabled = &enabled
	trait.ctx = context.TODO()
	trait.client = client

	environment := &Environment{
		Catalog: NewCatalog(context.TODO(), nil),
		Integration: &v1.Integration{
			Status: v1.IntegrationStatus{
				Phase: v1.IntegrationPhaseDeploying,
			},
		},
		IntegrationKit: &v1.IntegrationKit{
			Status: v1.IntegrationKitStatus{
				Phase: v1.IntegrationKitPhaseReady,
			},
		},
		Resources: kubernetes.NewCollection(),
	}

	return trait, environment
}