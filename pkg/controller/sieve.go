package controller

import (
	kubedump "kubedump/pkg"
	"kubedump/pkg/filter"
)

type Sieve interface {
	Matches(resource kubedump.HandledResource) bool

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

func (sieve *sieve) Matches(resource kubedump.HandledResource) bool {
	return sieve.filter.Matches(resource)
}

func (sieve *sieve) Expression() filter.Expression {
	return sieve.filter
}
