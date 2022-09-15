# Filters
One of the more powerful capabilities of kubedump is to filter out what you don't want or need to look at. For example,
if you only care about resources in the `middle-earth` but not in any other namespaces, you can use the simple filter
`namespace middle-earth` to only select resources in the target namespace. See below for more detailed explanations on
all the features provided by filters.

## Pattern Matching
Kubedump supports using the `*` wildcard when matching values. For example, the pod pattern `middle-earth/*` matches any
pod under the `middle-earth` namespace.

## Logical Operators
Kubedump filters support the 3 most basic logical operators `and`, `or`, and `not`, and are used to either chain
multiple filter expressions together or negate an expression. For example, if you want to only watch pods in the
`middle-earth` namespaces but whose name is not `sauron` you could use the filter
`pod middle-earth/* and not pod middle-earth/sauron`.

## Resource Expressions

The most important thing to know about resource expressions is that any resource that does not match the specified type
will immediately return false regardless of the name or namespace. This means that when filtering on more than one
resource type, you must chain them with an `or` operator or the expression will ALWAYS be false. For example, the filter
`pod default/* and job default/*` will always return false because no resource can be both a pod and a job; however,
`pod default/* or job default/*` will select all pods and jobs in the `default` namespace. Similarly, the expression
`pod default/* and not job default/*` is equivalent to `pod default/*` since anything that is a pod will not be a job.

### Namespaced Resources
When filtering by a namespaced resource type, the pattern should follow the following format `[<namespace>/]<name>`. If
the namespace is omitted, kubedump will assume the default namespace. All resource expressions begin with the name of
the "lower flat case" of the resource. So to filter a `DaemonSet` (not yet supported) you could use the filter
`daemonset */*`.

### Non-Namespace Expressions
When filtering by a non-namespaced resource type, you just need to filter on the name of the resource. So using the
example above you can filter a namespace with the filter `namespace middle-earth`.

Many non-namespaced resources (like namespaces) can be used to filter other resources. So the above filter
`namespace middle-earth` will not just match the namespace resource but all resources that fall under that resource.

## Label Expressions
You may also filter on resource labels. With labels expressions you can specify some interesting labels or selectors.
Label selector patterns should follow the following format `labels [[key]=[value]]...`. This means that all the
following are valid label expressions:

| expression                         | what will be matched                                            |
|------------------------------------|-----------------------------------------------------------------|
| `label race=hobbit family=baggins` | any resource with the labels "race=hobbit" and "family=baggins" |
| `label race=hobbit`                | any resource with the label "race=hobbit"                       |
| `label race=`                      | any resource with an empty "race" label                         |
| `label`                            | anything                                                        |


Empty label names are not allowed since they are not valid k8s labels.

You might see from the table above, that a resource may have more labels that just those requested, but must have *at
least* the those labels. This means that the filter `label race=hobbit family=baggins` would match a pod with the labels
`{"race": "hobbit", "family": "baggins", "job": "burgalar"}` but would not match a pod with the labels
`{"race": "hobbit", "family": "gamgee", "job": "gardener"}`.