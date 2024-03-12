# COSIGN Signing Documentation

Use this guide to sign an image with the Cosign tool by Sigstore, verify the image, deploy the image to a cluster, and (unsuccessfully attempt to) deploy an untrusted image to the same cluster.

### Prereqs

[Docker](https://www.docker.com/) is required for this guide. Additionally, an Azure subscription will be required if creating an Azure Container Registry as specified in the guide.

Kubectl and helm will also be required for installing cosign related charts to your cluster.

This guide assumes Ubuntu 22.04 Linux.

### Notes

If using a container registry other than ACR, feel free to ignore any steps related to the AZ cli and creating an ACR instance.

## Step 0: Log into Az CLI

```sh
az login
```

## Step 1: Set Variables

Please modify the below variables to your particular requirements.

```sh
# Azure details (replace with your own)
export SUBSCRIPTION=00000000-0000-0000-0000-000000000000
export RESOURCE_GROUP=cosign-demo
export LOCATION=eastus
export AKS_NAME=cosign-demo
export ACR_NAME=cosigncr
export REGISTRY=$ACR_NAME.azurecr.io
export REPO=app

# Image details
export TAG=trusted
export IMAGE=$REGISTRY/${REPO}:$TAG
export IMAGE_SOURCE=./docker/trusted/Dockerfile

# Untrusted image details
export UNTRUSTED_TAG=untrusted
export UNTRUSTED_IMAGE=$REGISTRY/${REPO}:$UNTRUSTED_TAG
export UNTRUSTED_SOURCE=./docker/untrusted/Dockerfile

# Kubernetes namespace to be secured
export SECURE_NAMESPACE=secure-namespace
```

## Step 2: Remove Old Resources

If this guide has been run before, it is important to remove old resources which will interfere with your ability to run through the guide again.

```sh
rm -rf ./ignored/cosign
docker image rm $IMAGE
docker image rm $UNTRUSTED_IMAGE
kubectl delete secret cosign-secret -n cosign-system 
```

## Step 3: Create Azure RG and ACR

```sh
az account set --subscription $SUBSCRIPTION # Set subscription
az group create --location $LOCATION --name $RESOURCE_GROUP # Create resource group
az acr create -g $RESOURCE_GROUP -n $ACR_NAME --sku Basic # Create container registry
```

## Optional Step: Create AKS Cluster

If not using your own kubernetes cluster, an AKS cluster can be created and attached to your ACR instance with the following command.

```sh
az aks create \
  --resource-group "${RESOURCE_GROUP}" \
  --name "${AKS_NAME}" \
  --node-vm-size Standard_DS3_v2 \
  --node-count 1 \
  --generate-ssh-keys \
  --attach-acr ${ACR_NAME}
```

## Step 4: Log into ACR

```sh
az acr login --name $ACR_NAME
```

## Step 5: Build and Push Docker Image, Grab Digest

Note that this assumes your newly pushed image will have its digest at the 0th index of the docker RepoDigests. If you have tagged a container image multiple times, this may not be the case. Modify the command to obtain your image digest if it is not at the 0th index.

```sh
docker build . -f $IMAGE_SOURCE -t $IMAGE
docker push $IMAGE
export DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' $IMAGE)
```

## Step 6: Build and Push Untrusted Docker Image

This image will not be signed and in effect will be an untrusted image which we can later attempt to deploy to our cluster.

```sh
docker build . -f $UNTRUSTED_SOURCE -t $UNTRUSTED_IMAGE
docker push $UNTRUSTED_IMAGE
export UNTRUSTED_DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' $UNTRUSTED_IMAGE)
```

## Step 7: Install Cosign

```sh
git clone https://github.com/sigstore/cosign
cd cosign
go install ./cmd/cosign

# Sometimes the binary will need to be moved into the bin to have the executable recognized in the path.
sudo mv $(go env GOPATH)/bin/cosign /bin
```

## Step 8: Create and Navigate to Gitignored Directory

```sh
mkdir ./ignored
mkdir ./ignored/cosign
cd ./ignored/cosign
```

## Step 9: Create Key Pair with Cosign

Leave the password blank.

```sh
cosign generate-key-pair
```

## Step 10: Sign Image with Private Key

Several questions will be prompted when running the sign command. Respond with "y" to both in order to sign the image.

```sh
cosign sign --key cosign.key $DIGEST
```

## Step 11: Verify Image with Public Key

```sh
cosign verify --key cosign.pub $DIGEST
```

## Step 12: Create Cosign System Namespace and Store Secret with Public Key

Assure your kubeconfig is set to the desired cluster before running any of the following `kubectl` commands.

```sh
kubectl create namespace cosign-system
kubectl create secret generic cosign-secret -n cosign-system \
--from-file=./cosign.pub
```

## Step 13: Install Cosign Helm Chart

```sh
helm repo add sigstore https://sigstore.github.io/helm-charts
helm repo update
helm install policy-controller -n cosign-system sigstore/policy-controller --devel
```

## Step 14: Create & Secure Image Integrity Namespace

```sh
kubectl create namespace $SECURE_NAMESPACE
kubectl label namespace $SECURE_NAMESPACE policy.sigstore.dev/include=true
```

## Step 15: Apply Cosign Policy

```sh
cat <<EOF > ./cosign-policy.yaml
apiVersion: policy.sigstore.dev/v1beta1
kind: ClusterImagePolicy
metadata:
  namespace: cosign-system
  name: image-policy
spec:
  images:
    - glob: "**"
  authorities:
    - key:
        secretRef:
          name: cosign-secret
EOF

kubectl apply -f ./cosign-policy.yaml
```

## Step 16: Deploy a Trusted Image

```sh
kubectl run -n $SECURE_NAMESPACE demo-signed --image=${DIGEST}
```

If the image was properly signed, you should see the pod be admitted into the cluster:

```sh
pod/demo-signed created
```

## Step 17: Attempt to Deploy an Untrusted Image

```sh
kubectl run -n $SECURE_NAMESPACE demo-unsigned --image=${UNTRUSTED_DIGEST}
```

For untrusted images, you will see a failure at the admission webhook which looks like the following message:

```sh
Error from server (BadRequest): admission webhook "policy.sigstore.dev" denied the request: validation failed: failed policy: image-policy: spec.containers[0].image
cosigncr.azurecr.io/app@sha256:... signature key validation failed for authority authority-0 for cosigncr.azurecr.io/app@sha256:...: no matching signatures
```

## Conclusion

You have successfully verified the integrity of an image and properly denied an untrusted image from entering your cluster!