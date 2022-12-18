package selector

type ResourceId struct {
	Kind      string
	Name      string
	Namespace string
}

type LabelSelector map[string]string

type Labels map[string]string
