# Open-Source Container Image Signing Demo

A supply chain attack is a cyber-attack which has been around for some time but has gained immense popularity in recent years. The attack leverages weaknesses in the security of the software supply chain, the chain of dependencies from which source code depends on for its functionality. The increase in popularity is in part due to the dramatic rise in automation surrounding CI/CD and the growth in popularity of open-source software. Supply chain attacks are frequently targeted at container images, where attackers attempt to subvert the integrity of software by inserting their own malicious code in place of genuine code, from where they are able to obtain secret credentials, steal computational power, or execute other potentially devastating code.

How are these supply chain attacks prevented? One methodology involves a public/private key strategy to guarantee the authenticity of container images before they are deployed. The strategy is straightforward. A development team creates and securely stores a private key which is used to sign authentic source code. The team disseminates the corresponding public key to interested consumers of the image. These parties can use the public key to verify the integrity of the software before deploying said code.

The same methodology can be applied to CI/CD systems/pipelines as well. After having built and pushed a container image, a pipeline can pull the secret key from a secure store and sign the container image. When the code is deploying, a trusted verification pod with access to the corresponding public key can verify the integrity of the signed image before completing the deployment. If the code is not authentic, the deployment will be denied, thus circumventing the attack.

## Guides/Demos

Currently, two paradigms of container image verification have gained popularity -- these projects are named _Notation_ (aka notary v2) and _Cosign_. Notary is a CNCF incubating project, and Cosign was developed by Sigstore, the special interest group on open source security under the Linux Foundation. Documentation for both of these technologies are included in the following section, and step-by-step guides on image verification can be found at the following links: [Notation](./docs/NOTATION.md) and [Cosign](./docs/COSIGN.md).

### Usage of Cosign vs. Notation

The kubernetes community appears to currently support Cosign over Notation for image verification ([1](https://github.com/sigstore/cosign/issues/423), [2](https://dev.to/timtsoitt/choose-cosign-over-notary-2146), [3](https://dlorenc.medium.com/notary-v2-and-cosign-b816658f044d#:~:text=Both%20tools%20support%20time%2Dstamping,cosign%20should%20switch%20to%20it.)). It should be noted that Notation is also a brand new technology and (with regards to its integration with AKS) is not recommended for use in production environments ([4](https://learn.microsoft.com/en-us/azure/aks/image-integrity?tabs=azure-cli#considerations-and-limitations)). Beyond container images, Consign can be used to verify the integrity of any OCI artifact, where Notation is currently constrained to the verification of container images. 

Based on this, as well as ease of configuring Cosign as compared to notation (see the difference between each step by step guide), it is likely in the best interest of developers to use Cosign for verifcation of their images. Cosign has robust documentation, a strong community, and was developed by leading security engineers around the world.

Notation does have better support in Azure as compared to Cosign. With this being said, all Notation integrations with Azure are in preview and should not be used with a production cluster.

## Associated Documentation

The Cosign and Notation step-by-step guides are based on the following documentation.

### Cosign Documentation

- https://docs.sigstore.dev/policy-controller/overview/
- https://github.com/sigstore/docs

### Notation Documentation

- https://ratify.dev/docs/1.0/quickstarts/ratify-on-azure/
- https://github.com/notaryproject/notation
- https://notaryproject.dev/

### AKS Integration Documentation

- https://learn.microsoft.com/en-us/azure/container-registry/container-registry-tutorial-sign-build-push
- https://learn.microsoft.com/en-us/azure/aks/image-integrity?tabs=azure-cli
