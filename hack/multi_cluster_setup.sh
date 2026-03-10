#
 # Derived from the ACM project
 # https://gitlab.eclipse.org/eclipse-research-labs/codeco-project/acm/-/blob/ACM-FC/LICENSE?ref_type=heads
 #
 #
 # Licensed under the Apache License, Version 2.0 (the "License");
 # you may not use this file except in compliance with the License.
 # You may obtain a copy of the License at
 #
 #     http://www.apache.org/licenses/LICENSE-2.0
 #
 # Unless required by applicable law or agreed to in writing, software
 # distributed under the License is distributed on an "AS IS" BASIS,
 # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 # See the License for the specific language governing permissions and
 # limitations under the License.
 #
 
 set -euo pipefail

############################
# 0. Variables
############################
hubctx="kind-hub"
c1ctx="kind-cluster1"
c2ctx="kind-cluster2"

c1="cluster1"
c2="cluster2"


############################
# 1. Initialize OCM Hub
############################
echo ">>> Initializing OCM Hub"
clusteradm init --wait --context "${hubctx}"

############################
# 2. Discover Hub API server
############################
echo ">>> Discovering Hub API Server"
hub_apiserver="$(kubectl config view --raw --minify --context "${hubctx}" -o jsonpath='{.clusters[0].cluster.server}')"

############################
# 3. Generate Join Command
############################
echo ">>> Getting join token"
joincmd="$(clusteradm get token --context "${hubctx}" | awk '/^clusteradm join /{print; exit}')"

# Remove placeholder cluster-name flag
joincmd="$(echo "${joincmd}" | sed -E 's/--cluster-name[[:space:]]+<[^>]+>//g')"

############################
# 4. Register Managed Clusters
############################
echo ">>> Joining ${c1}"
eval "${joincmd} --hub-apiserver ${hub_apiserver} --force-internal-endpoint-lookup --wait --context ${c1ctx} --cluster-name ${c1}"

echo ">>> Joining ${c2}"
eval "${joincmd} --hub-apiserver ${hub_apiserver} --force-internal-endpoint-lookup --wait --context ${c2ctx} --cluster-name ${c2}"

############################
# 5. Accept Clusters on Hub
############################
echo ">>> Accepting managed clusters"
clusteradm accept --context "${hubctx}" --clusters "${c1},${c2}" --wait

############################
# 6. Label Managed Clusters
############################
echo ">>> Labeling clusters"
kubectl --context "${hubctx}" label managedcluster "${c1}" codeco-enabled=true --overwrite
kubectl --context "${hubctx}" label managedcluster "${c2}" codeco-enabled=true --overwrite

# Add clusters to the default ManagedClusterSet (required for Placement to select them)
kubectl --context "${hubctx}" label managedcluster "${c1}" cluster.open-cluster-management.io/clusterset=default --overwrite
kubectl --context "${hubctx}" label managedcluster "${c2}" cluster.open-cluster-management.io/clusterset=default --overwrite

kubectl --context "${hubctx}" get managedclusters -o wide

############################
# 7. Install Open Cluster Management App CRDs
############################
echo ">>> Installing OCM App CRDs"
kubectl apply -f https://raw.githubusercontent.com/open-cluster-management-io/multicloud-operators-subscription/main/deploy/hub-common/apps.open-cluster-management.io_placementrules_crd.yaml

############################
# 8. Install Governance Policy Framework on Hub
############################
echo ">>> Installing Governance Policy Framework addon"
clusteradm install hub-addon --names governance-policy-framework --context "${hubctx}"

############################
# 9. Enable Governance Addon (sync components)
############################
echo ">>> Enabling Governance addon sync to hub"
clusteradm addon enable --names governance-policy-framework \
  --clusters "${c1}" \
  --annotate addon.open-cluster-management.io/on-multicluster-hub=true \
  --context "${hubctx}"

clusteradm addon enable --names governance-policy-framework \
  --clusters "${c2}" \
  --annotate addon.open-cluster-management.io/on-multicluster-hub=true \
  --context "${hubctx}"

echo ">>> Enabling Governance addon on managed clusters"
clusteradm addon enable --names governance-policy-framework --clusters "${c1}" --context "${hubctx}"
clusteradm addon enable --names governance-policy-framework --clusters "${c2}" --context "${hubctx}"

############################
# 10. Install Config Policy Controller
############################
echo ">>> Enabling config-policy-controller"
clusteradm addon enable --names config-policy-controller --clusters "${c1}" --context "${hubctx}"
clusteradm addon enable --names config-policy-controller --clusters "${c2}" --context "${hubctx}"

############################
# 11. Create Policies Namespace + Binding
############################
echo ">>> Creating policies namespace and binding"
kubectl --context "${hubctx}" create namespace policies || true

kubectl --context "${hubctx}" apply -f - <<EOF
apiVersion: cluster.open-cluster-management.io/v1beta2
kind: ManagedClusterSetBinding
metadata:
  name: default
  namespace: policies
spec:
  clusterSet: default
EOF

############################
# 12. Label All Clusters (bulk safety)
############################
kubectl --context "${hubctx}" label managedclusters --all codeco-enabled=true --overwrite

############################
# 13. Install Policy Generator Plugin
############################
echo ">>> Installing Policy Generator plugin"
PLUGIN_DIR="$HOME/.config/kustomize/plugin/policy.open-cluster-management.io/v1/policygenerator"
mkdir -p "$PLUGIN_DIR"

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
esac

# Map OS name to release binary naming
case "$OS" in
  darwin) OS_NAME="darwin" ;;
  linux) OS_NAME="linux" ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

BINARY_NAME="${OS_NAME}-${ARCH}-PolicyGenerator"
echo ">>> Downloading PolicyGenerator for ${OS_NAME}/${ARCH}"

curl -sL "https://github.com/open-cluster-management-io/policy-generator-plugin/releases/latest/download/${BINARY_NAME}" \
  -o "$PLUGIN_DIR/PolicyGenerator"

chmod +x "$PLUGIN_DIR/PolicyGenerator"

"$PLUGIN_DIR/PolicyGenerator" --version

echo ">>> Setup Complete!"
