package controller

import "kubedump/pkg/filter"

type Sieve interface {
	Matches(obj interface{}) bool

	Expression() filter.Expression
}

func NewSieve(filter filter.Expression) (Sieve, error) {
	return &sieve{
		filter: filter,
	}, nil
}

type sieve struct {
	filter filter.Expression
}

func (sieve *sieve) Matches(obj interface{}) bool {
	// todo: handle parent filters
	return sieve.filter.Matches(obj)
}

func (sieve *sieve) Expression() filter.Expression {
	return sieve.filter
}
