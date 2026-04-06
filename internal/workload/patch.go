package workload

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// MergeFrom computes a strategic merge patch between the original snapshot
// (taken before mutations) and the current state of the object.
func MergeFrom(before, after runtime.Object) ([]byte, error) {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return nil, err
	}

	afterJSON, err := json.Marshal(after)
	if err != nil {
		return nil, err
	}

	return strategicpatch.CreateTwoWayMergePatch(beforeJSON, afterJSON, before)
}
