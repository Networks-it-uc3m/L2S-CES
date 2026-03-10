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

package env

import (
	"os"
)

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func GetDefaultSDNPort() string {
	return getEnv("DEFAULT_SDN_PORT", "30808")
}

func GetDefaultDNSGRPCPort() string {
	return getEnv("DEFAULT_DNS_GRPC_PORT", "30818")
}

func GetDefaultDNSPort() string {
	return getEnv("DEFAULT_DNS_PORT", "30053")
}

func GetDefaultOFPort() string {
	return getEnv("DEFAULT_OF_PORT", "30663")
}

func GetMultiDomainClientType() string {
	return getEnv("MULTI_DOMAIN_CLIENT_TYPE", "ocm")
}
