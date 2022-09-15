# Storage
Kubedump stores the collected resources, logs, and events on the local filesystem whether this is the k8s cluster host
or the user's machine. If on the k8s cluster, the root directory will be at `/var/lib/kubedump`. By default this will be
`./kubedump` on the user's machine, but this can be changed via the `--destination` argument.

## Resource Storage
Resource directory will generally follow the following format `kubedump/<namespace>/<resource-kind>/<resource-name>/`.
Any resource files such as logs, descriptions, events, or related resources will be stored here. Non-namespaced resource
will ommit the namespace: `kubedump/<resource-kind>/<resource-name>`.

### Resource Files
The below table describe where different resource information is store under each resource directory:

| what is stored  | path                   | notes                |
|-----------------|------------------------|----------------------|
| resource events | <resource-name>.events |                      |
| container logs  | <container-name>.logs  | only present in pods |
| resource yaml   | <resource-name.yaml>   |                      |

### Ownership
When a resource has listed ownership references, a symlink to the resource is created in the owner's resource directory.
For example, the pod `example-job-pod-xxxxx` which is owned by a job `example-job` in the namespace `default` will have
a directory at `kubedump/default/pod/example-job-pod-xxxxx` but also a symlink to the resource directory at
`kubedump/default/job/example-job/pod/example-job-pod-xxxxx`.