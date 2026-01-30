/*
Copyright 2024.

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

package v2

import (
	v1 "github.com/openshift/dpu-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this DataProcessingUnit (v2) to the Hub version (v1).
func (src *DataProcessingUnit) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1.DataProcessingUnit)

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec conversion - v2 has more fields, v1 is simpler
	dst.Spec.NodeName = src.Spec.NodeName
	dst.Spec.Vendor = src.Spec.Vendor
	dst.Spec.DpuMode = src.Spec.IsDpuSide

	// Status conversion
	for _, cond := range src.Status.Conditions {
		dst.Status.Conditions = append(dst.Status.Conditions, cond)
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1) to this version (v2).
func (dst *DataProcessingUnit) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1.DataProcessingUnit)

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec conversion
	dst.Spec.NodeName = src.Spec.NodeName
	dst.Spec.Vendor = src.Spec.Vendor
	dst.Spec.IsDpuSide = src.Spec.DpuMode

	// Status conversion
	for _, cond := range src.Status.Conditions {
		dst.Status.Conditions = append(dst.Status.Conditions, cond)
	}

	// Default phase based on conditions
	dst.Status.Phase = DpuPhaseDiscovered

	return nil
}
