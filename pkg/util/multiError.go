package util

import (
	"strings"
)

//MultiErrorer Interface for the MutiError
type MultiErrorer interface {
	AddError(err error)
	GetErrors() []error
	Error() string
	AppendErrors(errs []error)
	AppendMultiErrorer(other MultiErrorer)
	IsNil() bool
}

//MultiError allows stacking multiple errors into one error
type MultiError struct {
	errors []error
}

//NewMultiError returns a new MultiErrorer implementation
func NewMultiError() MultiErrorer {
	return &MultiError{}
}

//IsNil returns true if there are no errors
func (me *MultiError) IsNil() bool {
	return len(me.errors) == 0
}

//AddError to the multierror object
func (me *MultiError) AddError(err error) {
	if err == nil {
		return
	}
	me.errors = append(me.errors, err)
}

//AppendErrors to array of contained errors
func (me *MultiError) AppendErrors(errs []error) {
	me.errors = append(me.errors, errs...)
}

//AppendMultiErrorer errors to contained errors
func (me *MultiError) AppendMultiErrorer(other MultiErrorer) {
	me.AppendErrors(other.GetErrors())
}

//GetErrors from the multierror object
func (me *MultiError) GetErrors() []error {
	return me.errors
}

//Error message of all errors concatanated
func (me *MultiError) Error() string {
	if len(me.GetErrors()) == 0 {
		return ""
	}
	errors := []string{}
	for _, err := range me.errors {
		errors = append(errors, err.Error())
	}
	return strings.Join(errors, ": ")
}
