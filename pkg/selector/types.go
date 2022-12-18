package selector

type ResourceWrapper struct {
	Kind      string
	Name      string
	Namespace string

	Resource interface{}
}

type LabelSelector map[string]string

type Labels map[string]string
