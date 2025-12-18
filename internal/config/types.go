package config

// Profile represents a single server configuration
type Profile struct {
	Host       string `json:"host"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	SSHKey     string `json:"sshKey"`
	Port       int    `json:"port"`
	Protocol   string `json:"protocol"`
	RemotePath string `json:"remotePath"`
	Context    string `json:"context"`
}

// Config represents the entire configuration file
type Config struct {
	Profiles map[string]Profile
}

// Validate checks if all required fields are present and valid
func (p *Profile) Validate() error {
	if p.Host == "" {
		return ErrMissingHost
	}
	if p.Username == "" {
		return ErrMissingUsername
	}
	// Validate protocol
	if p.Protocol != "ftp" && p.Protocol != "sftp" {
		return ErrInvalidProtocol
	}
	// Validate port
	if p.Port < 1 || p.Port > 65535 {
		return ErrInvalidPort
	}
	// For SFTP/SSH protocols, require either password or SSH key
	if p.Protocol == "sftp" {
		if p.Password == "" && p.SSHKey == "" {
			return ErrMissingPasswordOrKey
		}
	} else {
		// For FTP, password is still required
		if p.Password == "" {
			return ErrMissingPassword
		}
	}
	if p.Context == "" {
		return ErrMissingContext
	}
	return nil
}

// SetDefaults applies default values for optional fields
func (p *Profile) SetDefaults() {
	if p.Port == 0 {
		if p.Protocol == "sftp" {
			p.Port = 22
		} else {
			p.Port = 21
		}
	}
	if p.Protocol == "" {
		p.Protocol = "ftp"
	}
	if p.RemotePath == "" {
		p.RemotePath = "/"
	}
}
