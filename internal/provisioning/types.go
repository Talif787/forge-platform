package provisioning

type ProvisionTenantInput struct {
	Slug        string
	MaxServices int
}

type ProvisionTenantResult struct {
	Namespace string
	Endpoint  string
}

type NamespaceRequest struct{ Slug string }
type NamespaceResult struct{ Name string }

type QuotaRequest struct {
	Slug        string
	MaxServices int
}

type DNSRequest struct{ Slug string }
type DNSResult struct{ FQDN string }

type SecretRequest struct{ Slug string }
type FinalizeRequest struct{ Slug string }
