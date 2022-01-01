// Package accountInterpreter
//
// @author: xwc1125
package accountInterpreter

import "strings"

func isSubDomain(domain, subdomain string) bool {
	return strings.HasSuffix(subdomain, "."+domain)
}