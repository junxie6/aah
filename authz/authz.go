// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/security source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package authz

import (
	"errors"

	"aahframework.org/config.v0"
	"aahframework.org/security.v0/authc"
)

var (
	// ErrAuthorizerIsNil error is return when authorizer is nil in the auth scheme.
	ErrAuthorizerIsNil = errors.New("security: authorizer is nil")
)

// Authorizer interface is gets implemented by user application to provide Subject's
// (aka 'application user') access control information.
type Authorizer interface {
	// Init method gets called by framework during an application start.
	Init(appCfg *config.Config) error

	// GetAuthorizationInfo method gets called after authentication is successful
	// to get Subject's aka User access control information such as roles and permissions.
	GetAuthorizationInfo(authcInfo *authc.AuthenticationInfo) *AuthorizationInfo
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Unexported methods
//___________________________________

// acquirePermission method gets from pool or creates an `Permission` instance
// with zero values.
func acquirePermission() *Permission {
	return permissionPool.Get().(*Permission)
}

// releasePermission method resets and puts back to pool for repurpose.
func releasePermission(permissions ...*Permission) {
	for _, p := range permissions {
		if p != nil {
			p.Reset()
			permissionPool.Put(p)
		}
	}
}
