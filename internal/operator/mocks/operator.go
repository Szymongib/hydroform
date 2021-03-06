// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"

	types "github.com/kyma-incubator/hydroform/types"
)

// Operator is an autogenerated mock type for the Operator type
type Operator struct {
	mock.Mock
}

// Create provides a mock function with given fields: providerType, configuration
func (_m *Operator) Create(providerType types.ProviderType, configuration map[string]interface{}) (*types.ClusterInfo, error) {
	ret := _m.Called(providerType, configuration)

	var r0 *types.ClusterInfo
	if rf, ok := ret.Get(0).(func(types.ProviderType, map[string]interface{}) *types.ClusterInfo); ok {
		r0 = rf(providerType, configuration)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.ClusterInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(types.ProviderType, map[string]interface{}) error); ok {
		r1 = rf(providerType, configuration)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Delete provides a mock function with given fields: state, providerType, configuration
func (_m *Operator) Delete(state *types.InternalState, providerType types.ProviderType, configuration map[string]interface{}) error {
	ret := _m.Called(state, providerType, configuration)

	var r0 error
	if rf, ok := ret.Get(0).(func(*types.InternalState, types.ProviderType, map[string]interface{}) error); ok {
		r0 = rf(state, providerType, configuration)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
