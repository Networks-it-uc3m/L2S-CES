// Copyright 2024 Universidad Carlos III de Madrid
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package l2sminterface

import (
	"fmt"

	l2sces "github.com/Networks-it-uc3m/l2sc-es/internal/server"
)

func GetWorkloadPatchInstructions(networkName string) []*l2sces.FieldPatch {

	patches := []*l2sces.FieldPatch{
		{Path: "spec.template.metadata.labels.l2sm", Value: "true"},
		{Path: "spec.template.metadata.labels.l2sm/app", Value: "<workload-name>"},
		{Path: "spec.template.metadata.annotations.l2sm/networks", Value: networkName},
		{Path: "spec.template.spec.containers[0].env[0].name", Value: "DNS_NAME"},
		{Path: "spec.template.spec.containers[0].env[0].value", Value: fmt.Sprintf("<workload-name>.%s.%s.l2sm", networkName, "inter")},
	}
	return patches
}
