package config

// DefaultTrustedProxies is used when TRUSTED_PROXIES is unset: forwarded
// headers are honored only from loopback, link-local, private and CGNAT
// ranges, which covers in-cluster gateways and VPC load balancers without
// configuration while keeping public direct clients from spoofing them.
var DefaultTrustedProxies = []string{
	"127.0.0.0/8",
	"::1/128",
	"169.254.0.0/16",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"100.64.0.0/10",
	"fd00::/8",
}
