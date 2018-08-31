package k8s

import "github.com/operator-framework/operator-sdk/pkg/sdk"

type Cruder struct {
}

func (c Cruder) Create(o sdk.Object) error {
	return sdk.Create(o)
}

func (c Cruder) Update(o sdk.Object) error {
	return sdk.Update(o)
}

func (c Cruder) Delete(object sdk.Object, opts ...sdk.DeleteOption) error {

	return sdk.Delete(object, opts...)
}

func (c Cruder) Get(object sdk.Object, opt ...sdk.GetOption) error {
	return sdk.Get(object, opt...)
}

func (c Cruder) List(namespace string, into sdk.Object, opt ...sdk.ListOption) error {
	return sdk.List(namespace, into, opt...)
}
