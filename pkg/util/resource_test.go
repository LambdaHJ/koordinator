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

package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	quotav1 "k8s.io/apiserver/pkg/quota/v1"

	"github.com/koordinator-sh/koordinator/apis/extension"
)

func TestMinResourceList(t *testing.T) {
	type args struct {
		a corev1.ResourceList
		b corev1.ResourceList
	}
	tests := []struct {
		name string
		args args
		want corev1.ResourceList
	}{
		{
			name: "min with an empty list",
			args: args{
				a: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("40Gi"),
				},
				b: corev1.ResourceList{},
			},
			want: corev1.ResourceList{},
		},
		{
			name: "min with an empty list in reverse order",
			args: args{
				a: corev1.ResourceList{},
				b: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("40Gi"),
				},
			},
			want: corev1.ResourceList{},
		},
		{
			name: "min with an zero list",
			args: args{
				a: NewZeroResourceList(),
				b: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("40Gi"),
				},
			},
			want: NewZeroResourceList(),
		},
		{
			name: "min with a regular list",
			args: args{
				a: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("50Gi"),
				},
				b: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("40Gi"),
				},
			},
			want: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("40Gi"),
			},
		},
		{
			name: "min with a regular list 1",
			args: args{
				a: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("50Gi"),
					extension.BatchCPU:    resource.MustParse("2000"),
					extension.BatchMemory: resource.MustParse("40Gi"),
				},
				b: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("12"),
					corev1.ResourceMemory: resource.MustParse("60Gi"),
				},
			},
			want: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("50Gi"),
			},
		},
		{
			name: "min with a regular list 2",
			args: args{
				a: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("50Gi"),
					extension.BatchCPU:    resource.MustParse("2000"),
					extension.BatchMemory: resource.MustParse("40Gi"),
				},
				b: corev1.ResourceList{
					corev1.ResourceCPU:          resource.MustParse("12"),
					corev1.ResourceMemory:       resource.MustParse("60Gi"),
					extension.ResourceNvidiaGPU: resource.MustParse("4"),
				},
			},
			want: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("50Gi"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MinResourceList(tt.args.a, tt.args.b)
			assert.Equal(t, tt.want, got)

			// compatibility check
			want1 := quotav1.Subtract(quotav1.Add(tt.args.a, tt.args.b), quotav1.Max(tt.args.a, tt.args.b))
			assert.True(t, IsResourceListEqualValue(want1, got), fmt.Sprintf("want: %+v, got: %+v", want1, got))
		})
	}
}

func TestIsResourceListEqualValue(t *testing.T) {
	type args struct {
		a corev1.ResourceList
		b corev1.ResourceList
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "two same resource list are equal",
			args: args{
				a: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("40Gi"),
				},
				b: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("40Gi"),
				},
			},
			want: true,
		},
		{
			name: "different resource quantity",
			args: args{
				a: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("40Gi"),
				},
				b: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
			},
			want: false,
		},
		{
			name: "different resource quantity 1",
			args: args{
				a: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("20"),
					corev1.ResourceMemory: resource.MustParse("40Gi"),
				},
				b: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
			},
			want: false,
		},
		{
			name: "different number of resource names",
			args: args{
				a: corev1.ResourceList{
					corev1.ResourceCPU:          resource.MustParse("20"),
					corev1.ResourceMemory:       resource.MustParse("40Gi"),
					extension.ResourceNvidiaGPU: resource.MustParse("4"),
				},
				b: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("20"),
					corev1.ResourceMemory: resource.MustParse("40Gi"),
				},
			},
			want: false,
		},
		{
			name: "different resource names",
			args: args{
				a: corev1.ResourceList{
					corev1.ResourceCPU:          resource.MustParse("20"),
					extension.ResourceNvidiaGPU: resource.MustParse("4"),
				},
				b: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("20"),
					corev1.ResourceMemory: resource.MustParse("40Gi"),
				},
			},
			want: false,
		},
		{
			name: "numerically equal ignoring zero values",
			args: args{
				a: corev1.ResourceList{
					corev1.ResourceCPU:          resource.MustParse("20"),
					corev1.ResourceMemory:       resource.MustParse("40Gi"),
					extension.ResourceNvidiaGPU: resource.MustParse("0"),
				},
				b: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("20"),
					corev1.ResourceMemory: resource.MustParse("40Gi"),
				},
			},
			want: true,
		},
		{
			name: "numerically equal ignoring zero values 1",
			args: args{
				a: NewZeroResourceList(),
				b: corev1.ResourceList{},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsResourceListEqualValue(tt.args.a, tt.args.b)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsResourceDiff(t *testing.T) {
	type args struct {
		old           corev1.ResourceList
		new           corev1.ResourceList
		resourceName  corev1.ResourceName
		diffThreshold float64
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "the new resource has big enough difference with the old one",
			args: args{
				old: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(0, resource.BinarySI),
				},
				new: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewQuantity(9, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(0, resource.BinarySI),
				},
				resourceName:  corev1.ResourceCPU,
				diffThreshold: 2,
			},
			want: true,
		},
		{
			name: "the new resource doesn't have big enough difference with the old one",
			args: args{
				old: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(0, resource.BinarySI),
				},
				new: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(0, resource.BinarySI),
				},
				resourceName:  corev1.ResourceCPU,
				diffThreshold: 2,
			},
			want: false,
		},
		{
			name: "the old resource doesn't have queryed resource type",
			args: args{
				old: corev1.ResourceList{
					// corev1.ResourceCPU:    *resource.NewQuantity(1, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(0, resource.BinarySI),
				},
				new: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(0, resource.BinarySI),
				},
				resourceName:  corev1.ResourceCPU,
				diffThreshold: 2,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsResourceDiff(tt.args.old, tt.args.new, tt.args.resourceName, tt.args.diffThreshold); got != tt.want {
				t.Errorf("IsResourceDiff() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuantityPtr(t *testing.T) {
	testQuantity := resource.MustParse("1000")
	testQuantityPtr := &testQuantity
	testQuantity1 := resource.MustParse("20Gi")
	testQuantityPtr1 := &testQuantity1
	testQuantityPtr2 := resource.NewQuantity(1000, resource.DecimalSI)
	testQuantity2 := *testQuantityPtr2
	tests := []struct {
		name string
		arg  resource.Quantity
		want *resource.Quantity
	}{
		{
			name: "quantity 0",
			arg:  testQuantity,
			want: testQuantityPtr,
		},
		{
			name: "quantity 1",
			arg:  testQuantity1,
			want: testQuantityPtr1,
		},
		{
			name: "quantity 2",
			arg:  testQuantity2,
			want: testQuantityPtr2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QuantityPtr(tt.arg)
			assert.Equal(t, tt.want, got)
		})
	}
}
