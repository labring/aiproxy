//go:build enterprise

package main

import "github.com/labring/aiproxy/core/enterprise"

func init() {
	enterpriseInitializer = enterprise.Initialize
}
