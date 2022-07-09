package kubedump

type Operator int

const (
	And Operator = iota
	Or
	Not
)

type NamespaceFilter struct {
	Namespace string
}

type PodFilter struct {
	Namespace string
	Name      string
}
