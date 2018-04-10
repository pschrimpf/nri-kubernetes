package metric

import (
	"testing"

	"github.com/newrelic/infra-integrations-sdk/metric"
	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/stretchr/testify/assert"
)

func TestK8sClusterMetricsManipulator(t *testing.T) {
	entityData, err := sdk.NewEntityData("fluentd-elasticsearch-jnqb7", "k8s:playground:kube-system:pod")
	if err != nil {
		t.Fatal()
	}
	metricSet := metric.MetricSet{
		"event_type":        "K8sPodSample",
		"podInfo.namespace": "kube-system",
		"podInfo.pod":       "fluentd-elasticsearch-jnqb7",
		"displayName":       "fluentd-elasticsearch-jnqb7",
		"entityName":        "k8s:playground:kube-system:pod:fluentd-elasticsearch-jnqb7",
		"clusterName":       "playground",
	}

	err = K8sClusterMetricsManipulator(metricSet, entityData.Entity, "modifiedClusterName")
	assert.Nil(t, err)

	expectedMetricSet := metric.MetricSet{
		"event_type":        "K8sPodSample",
		"podInfo.namespace": "kube-system",
		"podInfo.pod":       "fluentd-elasticsearch-jnqb7",
		"displayName":       "fluentd-elasticsearch-jnqb7",
		"entityName":        "k8s:playground:kube-system:pod:fluentd-elasticsearch-jnqb7",
		"clusterName":       "modifiedClusterName",
	}
	assert.Equal(t, expectedMetricSet, metricSet)
}

func TestK8sMetricSetTypeGuesser(t *testing.T) {
	guess, err := K8sMetricSetTypeGuesser("", "replicaset", "", nil)
	assert.Nil(t, err)
	assert.Equal(t, "K8sReplicasetSample", guess)
}

func TestK8sEntityMetricsManipulator(t *testing.T) {
	entityData, err := sdk.NewEntityData("fluentd-elasticsearch-jnqb7", "k8s:playground:kube-system:pod")
	if err != nil {
		t.Fatal()
	}
	metricSet := metric.MetricSet{
		"event_type":        "K8sPodSample",
		"podInfo.namespace": "kube-system",
		"podInfo.pod":       "fluentd-elasticsearch-jnqb7",
		"entityName":        "fluentd-elasticsearch-jnqb7",
		"clusterName":       "playground",
	}

	err = K8sEntityMetricsManipulator(metricSet, entityData.Entity, "")
	assert.Nil(t, err)

	expectedMetricSet := metric.MetricSet{
		"event_type":        "K8sPodSample",
		"podInfo.namespace": "kube-system",
		"podInfo.pod":       "fluentd-elasticsearch-jnqb7",
		"displayName":       "fluentd-elasticsearch-jnqb7",
		"entityName":        "fluentd-elasticsearch-jnqb7",
		"clusterName":       "playground",
	}
	assert.Equal(t, expectedMetricSet, metricSet)
}
