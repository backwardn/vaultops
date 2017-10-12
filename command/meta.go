package command

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/cli"
)

const (
	// EnvVaultAddress stores vault address env var name
	EnvVaultAddress = "VAULT_ADDR"
	// EnvVaultCACert stores vault CA cert env var name
	EnvVaultCACert = "VAULT_CACERT"
	// EnvVaultClientCert stores vault client cert env var name
	EnvVaultClientCert = "VAULT_CLIENT_CERT"
	// EnvVaultClientKey stores vault client key env var name
	EnvVaultClientKey = "VAULT_CLIENT_KEY"
	// EnvVaultInsecure stores vault skip SSL host verify env var name
	EnvVaultInsecure = "VAULT_SKIP_VERIFY"
	// EnvVaultTLSServerName stores vault TLS server name env var name
	EnvVaultTLSServerName = "VAULT_TLS_SERVER_NAME"
	// EnvVaultToken stores vault token env var name
	EnvVaultToken = "VAULT_TOKEN"
	// localPath points to vault keys
	localDir  = ".local"
	localFile = "vault.json"
)

// VaultKeys storea parsed out vault server keys and tokens
type VaultKeys struct {
	// RootToken stores vault root token
	RootToken string `json:"root_token,omitempty"`
	// MasterKeys stores master keys used to unseal vault servers
	MasterKeys []string `json:"master_keys,omitempty"`
	// Token stores token issues by vault server
	Token string `json:"token,omitempty"`
}

// VaultMount stores vault Mount Info
type VaultMount struct {
	// Path is a vault mount path
	Path string
	// MountInput contains mount configuration
	MountInput *api.MountInput
}

// VaultRole stores vault role
type VaultRole struct {
	// Backend is vault backedn
	Backend string
	// Name is a vault role name
	Name string
	// Config is vault role config
	Config map[string]interface{}
}

// VaultCert is SSL certificate generated by vault
type VaultCert struct {
	// Backend is vault backedn
	Backend string
	// Type is type of SSL certificate (internal or exported)
	Type string
	// Root SSL certificate
	Root bool
	// Role is vault role
	Role string
	// Config is SSL certificate config
	Config map[string]interface{}
	// Store flag to request vault storage
	Store bool
}

// VaultBackend is vault backend
type VaultBackend struct {
	// Name is vault backend name
	Name string
	// Roles stores VaultRoles
	Roles []*VaultRole
	// Certs store VaultCerts
	Certs []*VaultCert
}

// VaultPolicy is vault policy
type VaultPolicy struct {
	// Name is a name of a vault policy
	Name string
	// Rules are vault policy rules
	Rules string
}

// FlagSetFlags is an enum to define what flags are present in the
// default FlagSet returned by Meta.FlagSet.
type FlagSetFlags uint

const (
	// FlagSetNone allows to implement FlagSet enum
	FlagSetNone FlagSetFlags = 0
	// FlagSetServer allows to provide FlagSet flags
	FlagSetServer FlagSetFlags = 1 << iota
	// FlagSetDefault allows to use  default FlagSet flags
	FlagSetDefault = FlagSetServer
)

// Meta contains meta-options used by almost every command
// This is a stipped version of vault meta struct
type Meta struct {
	// vault client token
	token string
	// UI is the cli UI
	UI cli.Ui
	// These are set by the command line flags.
	flagAddress    string
	flagCACert     string
	flagCAPath     string
	flagClientCert string
	flagClientKey  string
	flagInsecure   bool
}

// FlagSet returns a FlagSet with the common flags that every
// command implements.
func (m *Meta) FlagSet(name string, fs FlagSetFlags) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)

	// FlagSetServer tells us to enable the settings for selecting
	// the server information.
	if fs&FlagSetServer != 0 {
		f.StringVar(&m.flagAddress, "address", "", "")
		f.StringVar(&m.flagCACert, "ca-cert", "", "")
		f.StringVar(&m.flagCAPath, "ca-path", "", "")
		f.StringVar(&m.flagClientCert, "client-cert", "", "")
		f.StringVar(&m.flagClientKey, "client-key", "", "")
		f.BoolVar(&m.flagInsecure, "insecure", false, "")
		f.BoolVar(&m.flagInsecure, "tls-skip-verify", false, "")
	}

	return f
}

// Config returns vault *api.Config or fails with error
func (m *Meta) Config(address string) (*api.Config, error) {
	// default vault config
	config := api.DefaultConfig()

	err := config.ReadEnvironment()
	if err != nil {
		return nil, fmt.Errorf("Error reading environment: %v", err)
	}

	if m.flagAddress != "" {
		config.Address = m.flagAddress
	}
	// override the flag value
	if address != "" {
		config.Address = address
	}

	// If we need custom TLS configuration, then set it
	if m.flagCACert != "" || m.flagCAPath != "" || m.flagClientCert != "" || m.flagClientKey != "" || m.flagInsecure {
		t := &api.TLSConfig{
			CACert:        m.flagCACert,
			CAPath:        m.flagCAPath,
			ClientCert:    m.flagClientCert,
			ClientKey:     m.flagClientKey,
			TLSServerName: "",
			Insecure:      m.flagInsecure,
		}
		config.ConfigureTLS(t)
	}

	return config, nil
}

// Client initializes vault api.Client and returns it or fails with error
// or if mandatory options are missing. Ripped off (https://github.com/hashicorp/vault/blob/master/meta/meta.go#L74-L98)
func (m *Meta) Client(address, token string) (*api.Client, error) {
	config, err := m.Config(address)
	if err != nil {
		return nil, err
	}

	// Build the client
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	// retrieve token
	t := m.token

	// if none found check if token is already set
	if t == "" {
		t = client.Token()
	}

	// if still not found, lookup locally
	if t == "" {
		keys, err := readVaultKeys(filepath.Join(localDir, localFile))
		if err != nil {
			return nil, err
		}
		t = keys.RootToken
	}

	// if we pass in token, override VAULT_TOKEN
	if token != "" {
		t = token
		m.token = token
	}
	client.SetToken(t)

	return client, nil
}

// Token returns client token
func (m *Meta) Token() string {
	return m.token
}

// GeneralOptionsUsage returns the usage documentation for commonly
// available options this is ripped off (https://github.com/hashicorp/vault/blob/master/meta/meta.go#L177-L208)
func GeneralOptionsUsage() string {
	general := `
  -address=addr           The address of the Vault server
                          Overrides the VAULT_ADDR environment variable if set.

  -ca-cert=path           Path to a PEM encoded CA cert file to use to
                          verify the Vault server SSL certificate.
                          Overrides the VAULT_CACERT environment variable if set.

  -ca-path=path           Path to a directory of PEM encoded CA cert files
                          to verify the Vault server SSL certificate. If both
                          -ca-cert and -ca-path are specified, -ca-cert is used.
                          Overrides the VAULT_CAPATH environment variable if set.

  -client-cert=path       Path to a PEM encoded client certificate for TLS
                          authentication to the Vault server. Must also specify
                          -client-key. Overrides the VAULT_CLIENT_CERT
                          environment variable if set.

  -client-key=path        Path to an unencrypted PEM encoded private key
                          matching the client certificate from -client-cert.
                          Overrides the VAULT_CLIENT_KEY environment variable
                          if set.

  -tls-skip-verify        Do not verify TLS certificate. This is highly
                          not recommended. Verification will also be skipped
                          if VAULT_SKIP_VERIFY is set.
`

	return general
}