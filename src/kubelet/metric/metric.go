package metric

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	v1 "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"

	"github.com/newrelic/infra-integrations-beta/integrations/kubernetes/src/client"
	"github.com/newrelic/infra-integrations-beta/integrations/kubernetes/src/definition"
)

// StatsSummaryPath is the path where kubelet serves a summary with several information.
const StatsSummaryPath = "/stats/summary"

// GetMetricsData calls kubelet /stats/summary endpoint and returns unmarshalled response
func GetMetricsData(c client.HTTPClient) (v1.Summary, error) {
	resp, err := c.Do(http.MethodGet, StatsSummaryPath)
	if err != nil {
		return v1.Summary{}, err
	}
	defer resp.Body.Close() // nolint: errcheck
	if resp.StatusCode != http.StatusOK {
		return v1.Summary{}, fmt.Errorf("error calling kubelet endpoint. Got status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return v1.Summary{}, fmt.Errorf("error reading the response body of kubelet endpoint. Got error: %v", err.Error())
	}

	var summary = new(v1.Summary)
	err = json.Unmarshal(body, summary)
	if err != nil {
		return v1.Summary{}, fmt.Errorf("error unmarshaling the response body. Got error: %v", err.Error())
	}

	return *summary, nil

}

func fetchNodeStats(n v1.NodeStats) (definition.RawMetrics, string, error) {
	r := make(definition.RawMetrics)

	nodeName := n.NodeName
	if nodeName == "" {
		return r, "", fmt.Errorf("empty node identifier, possible data error in %s response", StatsSummaryPath)
	}

	r["nodeName"] = nodeName

	if n.CPU != nil {
		AddUint64RawMetric(r, "usageNanoCores", n.CPU.UsageNanoCores)
		AddUint64RawMetric(r, "usageCoreNanoSeconds", n.CPU.UsageCoreNanoSeconds)
	}

	if n.Memory != nil {
		AddUint64RawMetric(r, "memoryUsageBytes", n.Memory.UsageBytes)
		AddUint64RawMetric(r, "memoryAvailableBytes", n.Memory.AvailableBytes)
		AddUint64RawMetric(r, "memoryWorkingSetBytes", n.Memory.WorkingSetBytes)
		AddUint64RawMetric(r, "memoryRssBytes", n.Memory.RSSBytes)
		AddUint64RawMetric(r, "memoryPageFaults", n.Memory.PageFaults)
		AddUint64RawMetric(r, "memoryMajorPageFaults", n.Memory.MajorPageFaults)
	}

	if n.Network != nil {
		AddUint64RawMetric(r, "rxBytes", n.Network.RxBytes)
		AddUint64RawMetric(r, "txBytes", n.Network.TxBytes)
		if n.Network.RxErrors != nil && n.Network.TxErrors != nil {
			r["errors"] = *n.Network.RxErrors + *n.Network.TxErrors
		}
	}

	if n.Fs != nil {
		AddUint64RawMetric(r, "fsAvailableBytes", n.Fs.AvailableBytes)
		AddUint64RawMetric(r, "fsCapacityBytes", n.Fs.CapacityBytes)
		AddUint64RawMetric(r, "fsUsedBytes", n.Fs.UsedBytes)
		AddUint64RawMetric(r, "fsInodesFree", n.Fs.InodesFree)
		AddUint64RawMetric(r, "fsInodes", n.Fs.Inodes)
		AddUint64RawMetric(r, "fsInodesUsed", n.Fs.InodesUsed)
	}
	if n.Runtime != nil && n.Runtime.ImageFs != nil {
		AddUint64RawMetric(r, "runtimeAvailableBytes", n.Runtime.ImageFs.AvailableBytes)
		AddUint64RawMetric(r, "runtimeCapacityBytes", n.Runtime.ImageFs.CapacityBytes)
		AddUint64RawMetric(r, "runtimeUsedBytes", n.Runtime.ImageFs.UsedBytes)
		AddUint64RawMetric(r, "runtimeInodesFree", n.Runtime.ImageFs.InodesFree)
		AddUint64RawMetric(r, "runtimeInodes", n.Runtime.ImageFs.Inodes)
		AddUint64RawMetric(r, "runtimeInodesUsed", n.Runtime.ImageFs.InodesUsed)
	}

	return r, nodeName, nil
}

func fetchPodStats(pod v1.PodStats) (definition.RawMetrics, string, error) {
	r := make(definition.RawMetrics)

	if pod.PodRef.Name == "" || pod.PodRef.Namespace == "" {
		return r, "", fmt.Errorf("empty pod identifier, possible data error in %s response", StatsSummaryPath)
	}

	r["podName"] = pod.PodRef.Name
	r["namespace"] = pod.PodRef.Namespace

	if pod.Network != nil {
		AddUint64RawMetric(r, "rxBytes", pod.Network.RxBytes)
		AddUint64RawMetric(r, "txBytes", pod.Network.TxBytes)
		if pod.Network.RxErrors != nil && pod.Network.TxErrors != nil {
			r["errors"] = *pod.Network.RxErrors + *pod.Network.TxErrors
		}
	}

	rawEntityID := fmt.Sprintf("%s_%s", r["namespace"], r["podName"])

	return r, rawEntityID, nil
}

func fetchContainerStats(c v1.ContainerStats) (definition.RawMetrics, error) {
	r := make(definition.RawMetrics)

	if c.Name == "" {
		return r, fmt.Errorf("empty container identifier, possible data error in %s response", StatsSummaryPath)
	}
	r["containerName"] = c.Name

	if c.CPU != nil {
		AddUint64RawMetric(r, "usageNanoCores", c.CPU.UsageNanoCores)
	}
	if c.Memory != nil {
		AddUint64RawMetric(r, "usageBytes", c.Memory.UsageBytes)
	}

	return r, nil

}

// GroupStatsSummary groups specific data for pods, containers and node
func GroupStatsSummary(statsSummary v1.Summary) (definition.RawGroups, []error) {
	var errs []error
	var rawEntityID string
	g := definition.RawGroups{
		"pod":       {},
		"container": {},
		"node":      {},
	}

	rawNodeData, rawEntityID, err := fetchNodeStats(statsSummary.Node)
	if err != nil {
		errs = append(errs, err)
	} else {
		g["node"][rawEntityID] = rawNodeData
	}

	if statsSummary.Pods == nil {
		errs = append(errs, fmt.Errorf("pods data not found, possible data error in %s response", StatsSummaryPath))
		return g, errs
	}

PodListLoop:
	for _, pod := range statsSummary.Pods {
		rawPodMetrics, rawEntityID, err := fetchPodStats(pod)
		if err != nil {
			errs = append(errs, err)
			continue PodListLoop
		}
		g["pod"][rawEntityID] = rawPodMetrics

		if pod.Containers == nil {
			errs = append(errs, fmt.Errorf("containers data not found, possible data error in %s response", StatsSummaryPath))
			continue PodListLoop
		}

	ContainerListLoop:
		for _, container := range pod.Containers {
			rawContainerMetrics, err := fetchContainerStats(container)
			if err != nil {
				errs = append(errs, err)
				continue ContainerListLoop
			}
			rawContainerMetrics["podName"] = rawPodMetrics["podName"]
			rawContainerMetrics["namespace"] = rawPodMetrics["namespace"]

			rawEntityID = fmt.Sprintf("%s_%s_%s", rawPodMetrics["namespace"], rawPodMetrics["podName"], rawContainerMetrics["containerName"])

			g["container"][rawEntityID] = rawContainerMetrics
		}
	}

	return g, errs
}

// FromRawGroupsEntityIDGenerator generates an entityID from the pod name from kubelet. It's only used for k8s containers.
func FromRawGroupsEntityIDGenerator(key string) definition.EntityIDGeneratorFunc {
	// TODO:
	// what should be the result in case of error? Currently is "", but shouldn't be return rawEntityID?
	return func(groupLabel string, rawEntityID string, g definition.RawGroups) (string, error) {
		v, ok := g[groupLabel][rawEntityID][key]
		if !ok {
			return "", fmt.Errorf("%q not found for %q", key, groupLabel)
		}

		val, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("incorrect type of %q for %q", key, groupLabel)
		}
		return val, nil
	}
}

// FromRawEntityIDGroupEntityIDGenerator generates an entityID from the raw entity ID
// which is composed of namespace and pod name. It's used only for k8s pods.
func FromRawEntityIDGroupEntityIDGenerator(key string) definition.EntityIDGeneratorFunc {
	// TODO:
	// what should be the result in case of error? Currently is "", but shouldn't be return rawEntityID?
	return func(groupLabel string, rawEntityID string, g definition.RawGroups) (string, error) {
		toRemove, ok := g[groupLabel][rawEntityID][key]
		if !ok {
			return "", fmt.Errorf("%q not found for %q", key, groupLabel)
		}
		v := strings.TrimPrefix(rawEntityID, fmt.Sprintf("%s_", toRemove))

		if v == "" {
			return "", fmt.Errorf("generated entity ID is empty, skipping it")
		}

		return v, nil
	}
}

// FromRawGroupsEntityTypeGenerator generates the entity type using the value of the specified raw data key.
// If group label is different than "namespace" or "node", then entity type is composed of group label
// and specified key value (in case of error fetching the key, default value is used).
// Otherwise entity type is the same as group label.
func FromRawGroupsEntityTypeGenerator(key, defaultValue string) definition.EntityTypeGeneratorFunc {
	return func(groupLabel string, rawEntityID string, groups definition.RawGroups, clusterName string) (string, error) {
		var actualGroupLabel string
		switch groupLabel {
		case "namespace", "node":
			return fmt.Sprintf("k8s:%s:%s", clusterName, groupLabel), nil
		case "container":
			actualGroupLabel = "pod"
		default:
			actualGroupLabel = groupLabel
		}

		// TODO:
		// what should be the result in case of error and default value? Currently it is reported as "k8s:cluster_name::group_label"
		gl, ok := groups[groupLabel]
		if !ok {
			return fmt.Sprintf("k8s:%s:%s:%s", clusterName, defaultValue, actualGroupLabel), fmt.Errorf("%q not found", groupLabel)
		}
		en, ok := gl[rawEntityID]
		if !ok {
			return fmt.Sprintf("k8s:%s:%s:%s", clusterName, defaultValue, actualGroupLabel), fmt.Errorf("no entityID %q found for %q", rawEntityID, groupLabel)
		}

		v, ok := en[key]
		if !ok {
			return fmt.Sprintf("k8s:%s:%s:%s", clusterName, defaultValue, actualGroupLabel), fmt.Errorf("%q not found for %q", key, groupLabel)
		}

		v, ok = v.(string)
		if !ok {
			return fmt.Sprintf("k8s:%s:%s:%s", clusterName, defaultValue, actualGroupLabel), fmt.Errorf("incorrect type of %q for %q", key, groupLabel)
		}

		return fmt.Sprintf("k8s:%s:%s:%s", clusterName, v, actualGroupLabel), nil
	}
}

// AddUint64RawMetric adds a new metric to a RawMetrics if it exists
func AddUint64RawMetric(r definition.RawMetrics, name string, valuePtr *uint64) {
	if valuePtr != nil {
		r[name] = *valuePtr
	}
}
