# KubeDump
I found it *really* annoying and cumbersome to have to manually inspect all the different resources in a cluster at
runtime to debug my k8s clusters. I thought it would be a lot easier to pull all the interesting resources and events as
it happens, and then look at it later. I also wanted to be able to filter those resources so I didn't have to look at a
lot of unrelated pods or resources. This reminded me of `tcpdump` so I started working on `kubedump`.

## How to Run
There are two main ways to run kubedump, but both are availables using the `kubedump` binary.

### Running Locally
This is the simplest method; however, it will produce a lot more traffic over the network between you and your cluster,
so when you have a slow network connection you might want to run kubedump remotely.

To run locally you just need run `kubedump dump` via command line and you're off to the races. For more detailed usage
information run `kubedump dump --help`.

### Running Remotely
To run kubedump remotely, you will need to keep a few things in mind. When you run kubedump remotely, you are actually
just installing a helm chart that deploys the kubedump application inside your cluster and exposes it as a service. You
can verify that you have the necessary commands with `kubectl`:

```bash
# verify that you will be able to install the helm chart
kubectl auth can-i create namespaces
kubectl auth can-i create deployments
kubectl auth can-i create services
kubectl auth can-i create serviceaccounts
kubectl auth can-i create clusterroles
kubectl auth can-i create clusterrolebindings

# verify that you will be able to install the helm chart
kubectl auth can-i delete namspaces
kubectl auth can-i delete deployments
kubectl auth can-i delete services
kubectl auth can-i delete serviceaccounts
kubectl auth can-i delete clusterroles
kubectl auth can-i delete clusterrolebindings
```

If all report back that you have those permissions you are able to run kubedump remotely!

There are several sub-commands that you will need to know before you can start to run kubedump:

| Sub-Command | Use                                                       |
|-------------|-----------------------------------------------------------|
| create      | install the kubedump helm chart into your cluster         |
| start       | start watching the cluster                                |
| stop        | stop watching the cluster                                 |
| pull        | pull the collected data from the cluster as a tar archive |
| remove      | uninstall the kubedump helm chat from your cluster        |

**Note** that the `create` and `remove` sub-commands are not included in the `start` and `stop` sub-commands. This is done
to allow you to re-use a previous installation of kubedump, but also to allow you to use kubedump with the privelages
needed above as few times as possible if that is a concern for the cluster admin.

## Installation
For now, you will need to clone and build kubedump manually. You can do this with the following commands:

```bash
git clone https://github.com/joshmeranda/kubedump.git
make kubedump
```

You will need to have `make` and `go` installed on your machine for the build to work as expected.