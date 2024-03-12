# NOTATION w/ AKS Integration Signing Documentation

Use this guide to sign an image with Notation, verify the image, and deploy a trusted image to an AKS cluster. Please note that the AKS integration of notation is still in preview, and as such some of the following commands may fail and need to be tried again.

### Prereqs

[Docker](https://www.docker.com/) is required for this guide. Additionally, an Azure subscription will be required if creating an Azure Container Registry as specified in the guide.

Kubectl and helm will also be required for installing cosign related charts to your cluster.

This guide assumes Ubuntu 22.04 Linux.

## Step 0: Log into Az CLI

```sh
az login
```

## Step 1: Set Variables

Modify variables based on project need.

```sh
# Azure details (replace with your own)
export SUBSCRIPTION=00000000-0000-0000-0000-000000000000
export RESOURCE_GROUP=notation-demo
export LOCATION=eastus
export KEY_VAULT_NAME=notation-kv
export TENANT_ID=72f988bf-86f1-41af-91ab-2d7cd011db47
export IDENTITY_NAME=notation-identity
export AKS_NAME=notation-demo

# Certificate name and subject details
export CERT_NAME=example-cert
export CERT_SUBJECT="CN=example.com,O=example,L=example,ST=WA,C=US"
export CERT_PATH=./${CERT_NAME}.pem
export STORE_TYPE="ca"
export STORE_NAME="example.com"

# Trusted image variables
export ACR_NAME=notationcr
export REGISTRY=$ACR_NAME.azurecr.io
export REPO=app
export TAG=trusted
export IMAGE=$REGISTRY/${REPO}:$TAG
export IMAGE_SOURCE=./docker/trusted/Dockerfile
export USER_ID=$(az ad signed-in-user show --query id -o tsv)

# Untrusted image variables
export UNTRUSTED_TAG=untrusted
export UNTRUSTED_IMAGE=$REGISTRY/${REPO}:$UNTRUSTED_TAG
export UNTRUSTED_SOURCE=./docker/untrusted/Dockerfile

# Desired namespace of ratify pod
export RATIFY_NAMESPACE="gatekeeper-system"
```

## Step 2: Remove Old Resources

```sh
docker image rm $IMAGE
docker image rm $UNTRUSTED_IMAGE
rm -rf ./ignored/notation
notation cert delete --type $STORE_TYPE --store $STORE_NAME $CERT_PATH
```

## Step 3: Create RG, KV, and ACR

```sh
az account set --subscription $SUBSCRIPTION # Set subscription
az group create --location $LOCATION --name $RESOURCE_GROUP # Create resource group
az provider register -n Microsoft.KeyVault # Register key vault extension
az keyvault create --name $KEY_VAULT_NAME --resource-group $RESOURCE_GROUP --location $LOCATION # Create the key vault
az acr create -g $RESOURCE_GROUP -n $ACR_NAME --sku Basic # Create container registry
```

## Step 4: Set KV Access Policy

```sh
az keyvault set-policy -n $KEY_VAULT_NAME --certificate-permissions create get --key-permissions sign --object-id $USER_ID
```

## Step 5: Create and Navigate to Gitignored Directory

```sh
mkdir ./ignored
mkdir ./ignored/notation
cd ./ignored/notation
```

## Step 6: Create Certificate Policy

```sh
cat <<EOF > ./my_policy.json
{
    "issuerParameters": {
        "certificateTransparency": null,
        "name": "Self"
    },
    "keyProperties": {
        "exportable": false,
        "keySize": 2048,
        "keyType": "RSA",
        "reuseKey": true
    },
    "x509CertificateProperties": {
    "ekus": [
        "1.3.6.1.5.5.7.3.3"
    ],
    "keyUsage": [
        "digitalSignature"
    ],
    "subject": "$CERT_SUBJECT",
    "validityInMonths": 12
    }
}
EOF

az keyvault certificate create -n $CERT_NAME --vault-name $KEY_VAULT_NAME -p @my_policy.json
KEY_ID=$(az keyvault certificate show -n $CERT_NAME --vault-name $KEY_VAULT_NAME --query 'kid' -o tsv)
```

## Step 7: Log into ACR

```sh
az acr login --name $ACR_NAME
```

## Step 8: Build and Push Image

```sh
docker build . -f $IMAGE_SOURCE -t $IMAGE
docker push $IMAGE
DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' $IMAGE)
```

## Step 9: Sign Image

```sh 
notation sign --signature-format cose --id $KEY_ID --plugin azure-kv --plugin-config self_signed=true $DIGEST
notation ls $DIGEST
```

## Step 10: Load Public Key

```sh
az keyvault certificate download --name $CERT_NAME --vault-name $KEY_VAULT_NAME --file $CERT_PATH
notation cert add --type $STORE_TYPE --store $STORE_NAME $CERT_PATH
notation cert ls
```

## Step 11: Create Trust Policy

```sh
cat <<EOF > ./trustpolicy.json
{
    "version": "1.0",
    "trustPolicies": [
        {
            "name": "example",
            "registryScopes": [ "$REGISTRY/$REPO" ],
            "signatureVerification": {
                "level" : "strict" 
            },
            "trustStores": [ "$STORE_TYPE:$STORE_NAME" ],
            "trustedIdentities": [
                "x509.subject: $CERT_SUBJECT"
            ]
        }
    ]
}
EOF
```

## Step 12: Import and Verify Policy

```sh
notation policy import ./trustpolicy.json
notation policy show
notation verify $DIGEST
```

If all steps complete successfully, you should see a verification message from your command line output:

```Successfully verified signature for ...```

## Step 13: Create Untrusted Image 

```sh
docker build . -f $UNTRUSTED_SOURCE -t $UNTRUSTED_IMAGE
docker push $UNTRUSTED_IMAGE
UNTRUSTED_DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' $UNTRUSTED_IMAGE)
```

## Step 14: Attempt to Verify Untrusted Image

```sh
notation verify $UNTRUSTED_DIGEST
```

The images should not be able to be verified:

```Error: signature verification failed: no signature is associated with...```

## Step 15: Configure Workload Identity

```sh
az identity create --name "${IDENTITY_NAME}" --resource-group "${RESOURCE_GROUP}" --location "${LOCATION}" --subscription "${SUBSCRIPTION}"

export IDENTITY_OBJECT_ID="$(az identity show --name "${IDENTITY_NAME}" --resource-group "${RESOURCE_GROUP}" --query 'principalId' -otsv)"
export IDENTITY_CLIENT_ID=$(az identity show --name ${IDENTITY_NAME} --resource-group ${RESOURCE_GROUP} --query 'clientId' -o tsv)
```

## Step 16: Enable AKS Preview Extension

```sh
az extension add --name aks-preview
```

## Step 17: Register 'EnableWorkloadIdentityPreview' feature flag

```sh
az feature register --namespace "Microsoft.ContainerService" --name "EnableWorkloadIdentityPreview"
az provider register --namespace Microsoft.ContainerService
```

## Step 18: Create AKS Cluster with OIDC Issuer and Workload Identity

```sh
az aks create \
    --resource-group "${RESOURCE_GROUP}" \
    --name "${AKS_NAME}" \
    --node-vm-size Standard_DS3_v2 \
    --node-count 1 \
    --generate-ssh-keys \
    --enable-workload-identity \
    --attach-acr ${ACR_NAME} \
    --enable-oidc-issuer
```

## Step 19: Connect to AKS Cluster and Get OIDC Issuer

```sh
az aks get-credentials --resource-group ${RESOURCE_GROUP} --name ${AKS_NAME}

export AKS_OIDC_ISSUER="$(az aks show -n ${AKS_NAME} -g ${RESOURCE_GROUP} --query "oidcIssuerProfile.issuerUrl" -otsv)"
```

## Step 20: Create Role Assignment

```sh
az role assignment create \
--assignee-object-id ${IDENTITY_OBJECT_ID} \
--role acrpull \
--scope subscriptions/${SUBSCRIPTION}/resourceGroups/${RESOURCE_GROUP}/providers/Microsoft.ContainerRegistry/registries/${ACR_NAME}
```

## Step 21: Show Registration Status and Wait Until Registered

```sh
az feature show --namespace "Microsoft.ContainerService" --name "EnableWorkloadIdentityPreview" -o table
```

__STOP!__ This may take up to 10 minutes to complete. Wait until the status becomes "Registered."

## Step 22: Create Federated Identity

```sh
az identity federated-credential create \
--name ratify-federated-credential \
--identity-name "${IDENTITY_NAME}" \
--resource-group "${RESOURCE_GROUP}" \
--issuer "${AKS_OIDC_ISSUER}" \
--subject system:serviceaccount:"${RATIFY_NAMESPACE}":"ratify-admin"
```

If you get an error on this step, attempt to recreate the identity and try again.

## Step 23: Obtain Vault URI

```sh
export VAULT_URI=$(az keyvault show --name ${KEY_VAULT_NAME} --resource-group ${RESOURCE_GROUP} --query "properties.vaultUri" -otsv)
```

## Step 24: Configure Managed Identity Permissions

```sh
az keyvault set-policy --name ${KEY_VAULT_NAME} \
--secret-permissions get \
--object-id ${IDENTITY_OBJECT_ID}
```

## Step 25: Install Gatekeeper

```sh
helm repo add gatekeeper https://open-policy-agent.github.io/gatekeeper/charts

helm install gatekeeper/gatekeeper  \
--name-template=gatekeeper \
--namespace gatekeeper-system --create-namespace \
--set enableExternalData=true \
--set validatingWebhookTimeoutSeconds=5 \
--set mutatingWebhookTimeoutSeconds=2
```

## Step 26: Install Ratify

```sh
helm repo add ratify https://deislabs.github.io/ratify

helm install ratify \
    ratify/ratify --atomic \
    --namespace ${RATIFY_NAMESPACE} --create-namespace \
    --set featureFlags.RATIFY_CERT_ROTATION=true \
    --set akvCertConfig.enabled=true \
    --set akvCertConfig.vaultURI=${VAULT_URI} \
    --set akvCertConfig.cert1Name=${CERT_NAME} \
    --set akvCertConfig.tenantId=${TENANT_ID} \
    --set oras.authProviders.azureWorkloadIdentityEnabled=true \
    --set azureWorkloadIdentity.clientId=${IDENTITY_CLIENT_ID}
```

## Step 27: Enforce Gatekeeper Policy

```sh
kubectl apply -f https://deislabs.github.io/ratify/library/default/template.yaml
kubectl apply -f https://deislabs.github.io/ratify/library/default/samples/constraint.yaml
```

## Step 28: Deploy Trusted Demo Pod

```sh
kubectl run demo-signed --image=${DIGEST}
```

## Step 29: Attempt to Deploy Untrusted Demo Pod

```sh
kubectl run demo-unsigned --image=${UNTRUSTED_DIGEST}
```