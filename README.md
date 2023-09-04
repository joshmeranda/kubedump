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

**Note** that the `create` and `remove` sub-commands are not included in the `start` and `stop` sub-commands. This is done
to allow you to re-use a previous installation of kubedump, but also to allow you to use kubedump with the privelages
needed above as few times as possible if that is a concern for the cluster admin.

## Building
For now, you will need to clone and build kubedump manually. You can do this with the following commands:

```bash
git clone https://github.com/joshmeranda/kubedump.git
make kubedump
```

You will need to have `make` and `go` installed on your machine for the build to work as expected.

## Testing

Integration testing uses `kind` to deploy test clusters