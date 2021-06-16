// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package tui

import (
	"os"

	"github.com/ScaleFT/sshkeys"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/crypto/ssh"
)

var (
	// ErrIdentityFileReadFailed is ErrIdentityFileReadFailed
	ErrIdentityFileReadFailed = errNS.NewType("id_read_failed", utils.ErrTraitPreCheck)
)

// SSHConnectionProps is SSHConnectionProps
type SSHConnectionProps struct {
	Password               string
	IdentityFile           string
	IdentityFilePassphrase string
}

// ReadIdentityFileOrPassword is ReadIdentityFileOrPassword
func ReadIdentityFileOrPassword(identityFilePath string, usePass bool) (*SSHConnectionProps, error) {
	// If identity file is not specified, prompt to read password
	if usePass {
		password := PromptForPassword("Input SSH password: ")
		return &SSHConnectionProps{
			Password: password,
		}, nil
	}

	// Identity file is specified, check identity file
	if len(identityFilePath) > 0 && utils.IsExist(identityFilePath) {
		buf, err := os.ReadFile(identityFilePath)
		if err != nil {
			return nil, ErrIdentityFileReadFailed.
				Wrap(err, "Failed to read SSH identity file '%s'", identityFilePath).
				WithProperty(SuggestionFromTemplate(`
Please check whether your SSH identity file {{ColorKeyword}}{{.File}}{{ColorReset}} exists and have access permission.
`, map[string]string{
					"File": identityFilePath,
				}))
		}

		// Try to decode as not encrypted
		_, err = ssh.ParsePrivateKey(buf)
		if err == nil {
			return &SSHConnectionProps{
				IdentityFile: identityFilePath,
			}, nil
		}

		// Other kind of error.. e.g. not a valid SSH key
		if _, ok := err.(*ssh.PassphraseMissingError); !ok {
			return nil, ErrIdentityFileReadFailed.
				Wrap(err, "Failed to read SSH identity file '%s'", identityFilePath).
				WithProperty(SuggestionFromTemplate(`
Looks like your SSH private key {{ColorKeyword}}{{.File}}{{ColorReset}} is invalid.
`, map[string]string{
					"File": identityFilePath,
				}))
		}

		// SSH key is passphrase protected
		passphrase := PromptForPassword("The SSH identity key is encrypted. Input its passphrase: ")
		if _, err := sshkeys.ParseEncryptedPrivateKey(buf, []byte(passphrase)); err != nil {
			return nil, ErrIdentityFileReadFailed.
				Wrap(err, "Failed to decrypt SSH identity file '%s'", identityFilePath)
		}

		return &SSHConnectionProps{
			IdentityFile:           identityFilePath,
			IdentityFilePassphrase: passphrase,
		}, nil
	}

	// No password, nor identity file were specified, check ssh-agent via the env SSH_AUTH_SOCK
	sshAuthSock := os.Getenv("SSH_AUTH_SOCK")
	if len(sshAuthSock) == 0 {
		return nil, ErrIdentityFileReadFailed.New("none of ssh password, identity file, SSH_AUTH_SOCK specified")
	}
	stat, err := os.Stat(sshAuthSock)
	if err != nil {
		return nil, ErrIdentityFileReadFailed.Wrap(err, "Failed to stat SSH_AUTH_SOCK file: '%s'", sshAuthSock)
	}
	if stat.Mode()&os.ModeSocket == 0 {
		return nil, ErrIdentityFileReadFailed.New("The SSH_AUTH_SOCK file: '%s' is not a valid unix socket file", sshAuthSock)
	}

	return &SSHConnectionProps{}, nil
}
