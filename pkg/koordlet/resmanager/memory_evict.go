/*
Copyright 2022 The Koordinator Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resmanager

import (
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/koordinator-sh/koordinator/apis/extension"
	"github.com/koordinator-sh/koordinator/pkg/features"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/metriccache"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/resourceexecutor"
)

const (
	memoryReleaseBufferPercent = 2
)

type MemoryEvictor struct {
	resManager    *resmanager
	lastEvictTime time.Time
}

type podInfo struct {
	pod     *corev1.Pod
	memUsed float64
}

func NewMemoryEvictor(mgr *resmanager) *MemoryEvictor {
	return &MemoryEvictor{
		resManager:    mgr,
		lastEvictTime: time.Now(),
	}
}

func (m *MemoryEvictor) memoryEvict() {
	klog.V(5).Infof("starting memory evict process")
	defer klog.V(5).Infof("memory evict process completed")

	if time.Now().Before(m.lastEvictTime.Add(time.Duration(m.resManager.config.MemoryEvictCoolTimeSeconds) * time.Second)) {
		klog.V(5).Infof("skip memory evict process, still in evict cooling time")
		return
	}

	nodeSLO := m.resManager.getNodeSLOCopy()
	if disabled, err := isFeatureDisabled(nodeSLO, features.BEMemoryEvict); err != nil {
		klog.Errorf("failed to acquire memory eviction feature-gate, error: %v", err)
		return
	} else if disabled {
		klog.V(4).Infof("skip memory evict, disabled in NodeSLO")
		return
	}

	thresholdConfig := nodeSLO.Spec.ResourceUsedThresholdWithBE
	thresholdPercent := thresholdConfig.MemoryEvictThresholdPercent
	if thresholdPercent == nil {
		klog.Warningf("skip memory evict, threshold percent is nil")
		return
	} else if *thresholdPercent < 0 {
		klog.Warningf("skip memory evict, threshold percent(%v) should greater than 0", *thresholdPercent)
		return
	}

	lowerPercent := int64(0)
	if thresholdConfig.MemoryEvictLowerPercent != nil {
		lowerPercent = *thresholdConfig.MemoryEvictLowerPercent
	} else {
		lowerPercent = *thresholdPercent - memoryReleaseBufferPercent
	}

	if lowerPercent >= *thresholdPercent {
		klog.Warningf("skip memory evict, lower percent(%v) should less than threshold percent(%v)", lowerPercent, *thresholdPercent)
		return
	}

	podMetrics := m.resManager.collectAllPodMetricsLast(metriccache.PodMemUsageMetric)
	node := m.resManager.statesInformer.GetNode()
	if node == nil {
		klog.Warningf("skip memory evict, Node %v is nil", m.resManager.nodeName)
		return
	}

	memoryCapacity := node.Status.Capacity.Memory().Value()
	if memoryCapacity <= 0 {
		klog.Warningf("skip memory evict, memory capacity(%v) should greater than 0", memoryCapacity)
		return
	}

	queryMeta, err := metriccache.NodeMemoryUsageMetric.BuildQueryMeta(nil)
	if err != nil {
		klog.Warningf("skip memory evict, get node query failed, error: %v", err)
		return
	}

	nodeMemoryUsed, err := m.resManager.collectorNodeMetricLast(queryMeta)
	if err != nil {
		klog.Warningf("skip memory evict, get node metrics error: %v", err)
		return
	}
	nodeMemoryUsage := int64(nodeMemoryUsed) * 100 / memoryCapacity
	if nodeMemoryUsage < *thresholdPercent {
		klog.V(5).Infof("skip memory evict, node memory usage(%v) is below threshold(%v)", nodeMemoryUsage, *thresholdPercent)
		return
	}

	klog.Infof("node(%v) MemoryUsage(%v): %.2f, evictThresholdUsage: %.2f, evictLowerUsage: %.2f",
		m.resManager.nodeName,
		nodeMemoryUsed,
		float64(nodeMemoryUsage)/100,
		float64(*thresholdPercent)/100,
		float64(lowerPercent)/100,
	)

	memoryNeedRelease := memoryCapacity * (nodeMemoryUsage - lowerPercent) / 100
	m.killAndEvictBEPods(node, podMetrics, memoryNeedRelease)
}

func (m *MemoryEvictor) killAndEvictBEPods(node *corev1.Node, podMetrics map[string]float64, memoryNeedRelease int64) {
	bePodInfos := m.getSortedBEPodInfos(podMetrics)
	message := fmt.Sprintf("killAndEvictBEPods for node(%v), need to release memory: %v", m.resManager.nodeName, memoryNeedRelease)
	memoryReleased := int64(0)

	var killedPods []*corev1.Pod
	for _, bePod := range bePodInfos {
		if memoryReleased >= memoryNeedRelease {
			break
		}

		killMsg := fmt.Sprintf("%v, kill pod: %v", message, bePod.pod.Name)
		killContainers(bePod.pod, killMsg)
		killedPods = append(killedPods, bePod.pod)
		if bePod.memUsed != 0 {
			memoryReleased += int64(bePod.memUsed)
		}
	}

	m.resManager.evictPodsIfNotEvicted(killedPods, node, resourceexecutor.EvictPodByNodeMemoryUsage, message)

	m.lastEvictTime = time.Now()
	klog.Infof("killAndEvictBEPods completed, memoryNeedRelease(%v) memoryReleased(%v)", memoryNeedRelease, memoryReleased)
}

func (m *MemoryEvictor) getSortedBEPodInfos(podMetricMap map[string]float64) []*podInfo {

	var bePodInfos []*podInfo
	for _, podMeta := range m.resManager.statesInformer.GetAllPods() {
		pod := podMeta.Pod
		if extension.GetPodQoSClassRaw(pod) == extension.QoSBE {
			info := &podInfo{
				pod:     pod,
				memUsed: podMetricMap[string(pod.UID)],
			}
			bePodInfos = append(bePodInfos, info)
		}
	}

	sort.Slice(bePodInfos, func(i, j int) bool {
		// TODO: https://github.com/koordinator-sh/koordinator/pull/65#discussion_r849048467
		// compare priority > podMetric > name
		if bePodInfos[i].pod.Spec.Priority != nil && bePodInfos[j].pod.Spec.Priority != nil && *bePodInfos[i].pod.Spec.Priority != *bePodInfos[j].pod.Spec.Priority {
			return *bePodInfos[i].pod.Spec.Priority < *bePodInfos[j].pod.Spec.Priority
		}
		if bePodInfos[i].memUsed != 0 && bePodInfos[j].memUsed != 0 {
			//return bePodInfos[i].podMetric.MemoryUsed.MemoryWithoutCache.Value() > bePodInfos[j].podMetric.MemoryUsed.MemoryWithoutCache.Value()
			return bePodInfos[i].memUsed > bePodInfos[j].memUsed
		} else if bePodInfos[i].memUsed == 0 && bePodInfos[j].memUsed == 0 {

			return bePodInfos[i].pod.Name > bePodInfos[j].pod.Name
		}
		return bePodInfos[j].memUsed == 0
	})

	return bePodInfos
}
